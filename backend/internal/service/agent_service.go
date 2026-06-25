package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/google/uuid"
)

type AgentService struct {
	db     *sqlite.DB
	docker *dockerclient.Client
	cfg    config.Config

	mu           sync.Mutex
	lastActivity map[string]time.Time
	startOnce    sync.Once
}

func NewAgentService(db *sqlite.DB, docker *dockerclient.Client, cfg config.Config) *AgentService {
	return &AgentService{
		db:           db,
		docker:       docker,
		cfg:          cfg,
		lastActivity: make(map[string]time.Time),
	}
}

const agentImage = "tamga-agent"
const idleTimeout = 30 * time.Minute

func (s *AgentService) ensureContainerRunning(ctx context.Context, containerName string, mounts []string) error {
	if s.docker.ContainerIsRunning(ctx, containerName) {
		return nil
	}
	if s.docker.ContainerExists(ctx, containerName) {
		if err := s.docker.StartContainer(ctx, containerName); err != nil {
			return fmt.Errorf("start existing agent container: %w", err)
		}
		slog.Info("agent container restarted", "container", containerName)
		return nil
	}
	if _, err := s.docker.CreateContainerOpts(ctx, containerName, agentImage, nil, "tamga-net", mounts); err != nil {
		return fmt.Errorf("create agent container: %w", err)
	}
	if err := s.docker.StartContainer(ctx, containerName); err != nil {
		return fmt.Errorf("start agent container: %w", err)
	}
	slog.Info("agent container created and started", "container", containerName)
	return nil
}

func (s *AgentService) trackActivity(containerName string) {
	s.mu.Lock()
	s.lastActivity[containerName] = time.Now()
	s.mu.Unlock()
}

func (s *AgentService) startIdleWatcher() {
	s.startOnce.Do(func() {
		go func() {
			for {
				time.Sleep(60 * time.Second)
				s.mu.Lock()
				for name, last := range s.lastActivity {
					if time.Since(last) > idleTimeout {
						slog.Info("stopping idle agent container", "container", name)
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						s.docker.StopContainer(ctx, name)
						s.docker.RemoveContainer(ctx, name)
						cancel()
						delete(s.lastActivity, name)
					}
				}
				s.mu.Unlock()
			}
		}()
	})
}

func (s *AgentService) Chat(ctx context.Context, projectID int64, message string) (*domain.AgentTask, error) {
	if s.docker == nil {
		return nil, fmt.Errorf("docker daemon not available")
	}

	project, err := s.db.FindProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	if project.Status != domain.ProjectStatusRunning {
		return nil, fmt.Errorf("project is not running")
	}

	taskID := uuid.New().String()
	task := &domain.AgentTask{
		ID:        taskID,
		ProjectID: projectID,
		Message:   message,
		Status:    domain.AgentTaskStatusPending,
	}
	if err := s.db.CreateAgentTask(task); err != nil {
		return nil, fmt.Errorf("create agent task: %w", err)
	}

	containerName := fmt.Sprintf("agent-%d", projectID)

	if err := s.ensureContainerRunning(ctx, containerName, nil); err != nil {
		slog.Error("ensure agent container running failed", "container", containerName, "error", err)
	}

	s.trackActivity(containerName)
	s.startIdleWatcher()

	go func() {
		if err := s.forwardTask(context.Background(), containerName, taskID, message, projectID, ""); err != nil {
			slog.Error("forward task to agent failed", "task_id", taskID, "error", err)
		}
	}()

	return task, nil
}

func (s *AgentService) GetTask(ctx context.Context, projectID int64, taskID string) (*domain.AgentTask, error) {
	task, err := s.db.FindAgentTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}
	if task.ProjectID != projectID {
		return nil, fmt.Errorf("task not found for this project")
	}

	containerName := fmt.Sprintf("agent-%d", projectID)
	if err := s.pollAgentTask(context.Background(), containerName, "9000", task); err != nil {
		slog.Warn("poll agent task error", "task_id", taskID, "error", err)
	}

	return task, nil
}

func (s *AgentService) ListTasks(ctx context.Context, projectID int64) ([]*domain.AgentTask, error) {
	return s.db.ListAgentTasks(projectID)
}

func (s *AgentService) IsAgentRunning(ctx context.Context, projectID int64) bool {
	containerName := fmt.Sprintf("agent-%d", projectID)
	return s.docker.ContainerIsRunning(ctx, containerName)
}

func (s *AgentService) StartAgent(ctx context.Context, projectID int64) error {
	containerName := fmt.Sprintf("agent-%d", projectID)
	return s.ensureContainerRunning(ctx, containerName, nil)
}

func (s *AgentService) StopAgent(ctx context.Context, projectID int64) error {
	containerName := fmt.Sprintf("agent-%d", projectID)

	s.mu.Lock()
	delete(s.lastActivity, containerName)
	s.mu.Unlock()

	if !s.docker.ContainerExists(ctx, containerName) {
		return nil
	}
	if err := s.docker.StopContainer(ctx, containerName); err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}
	if err := s.docker.RemoveContainer(ctx, containerName); err != nil {
		return fmt.Errorf("remove agent: %w", err)
	}
	slog.Info("agent container stopped", "container", containerName)
	return nil
}

func (s *AgentService) ChatForDir(ctx context.Context, dirID, dirPath, message string) (*domain.AgentTask, error) {
	if s.docker == nil {
		return nil, fmt.Errorf("docker daemon not available")
	}

	taskID := uuid.New().String()

	s.db.CreateAgentTask(&domain.AgentTask{
		ID:        taskID,
		ProjectID: 0,
		Message:   message,
		Status:    domain.AgentTaskStatusPending,
	})

	containerName := fmt.Sprintf("agent-%s", dirID)
	mounts := []string{fmt.Sprintf("%s:/workspace/%s", dirPath, dirID)}

	if err := s.ensureContainerRunning(ctx, containerName, mounts); err != nil {
		slog.Error("ensure agent container running failed", "container", containerName, "error", err)
	}

	s.trackActivity(containerName)
	s.startIdleWatcher()

	go func() {
		if err := s.forwardTask(context.Background(), containerName, taskID, message, 0, dirPath); err != nil {
			slog.Error("forward task to dir agent failed", "task_id", taskID, "error", err)
		}
	}()

	return &domain.AgentTask{ID: taskID}, nil
}

func (s *AgentService) GetTaskForDir(ctx context.Context, dirID, taskID string) (*domain.AgentTask, error) {
	task, err := s.db.FindAgentTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}

	containerName := fmt.Sprintf("agent-%s", dirID)
	if err := s.pollAgentTask(context.Background(), containerName, "9001", task); err != nil {
		slog.Warn("poll agent task error", "task_id", taskID, "error", err)
	}

	return task, nil
}

func (s *AgentService) ListTasksForDir(ctx context.Context) ([]*domain.AgentTask, error) {
	return s.db.ListAgentTasks(0)
}

func (s *AgentService) IsAgentRunningForDir(ctx context.Context, dirID string) bool {
	containerName := fmt.Sprintf("agent-%s", dirID)
	return s.docker.ContainerIsRunning(ctx, containerName)
}

func (s *AgentService) StartAgentForDir(ctx context.Context, dirID, dirPath string) error {
	containerName := fmt.Sprintf("agent-%s", dirID)
	mounts := []string{fmt.Sprintf("%s:/workspace/%s", dirPath, dirID)}
	return s.ensureContainerRunning(ctx, containerName, mounts)
}

func (s *AgentService) StopAgentForDir(ctx context.Context, dirID string) error {
	containerName := fmt.Sprintf("agent-%s", dirID)

	s.mu.Lock()
	delete(s.lastActivity, containerName)
	s.mu.Unlock()

	if !s.docker.ContainerExists(ctx, containerName) {
		return nil
	}
	if err := s.docker.StopContainer(ctx, containerName); err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}
	if err := s.docker.RemoveContainer(ctx, containerName); err != nil {
		return fmt.Errorf("remove agent: %w", err)
	}
	slog.Info("agent container stopped", "container", containerName)
	return nil
}

func (s *AgentService) startAgentContainer(ctx context.Context, containerName string, mounts []string) error {
	if _, err := s.docker.CreateContainerOpts(ctx, containerName, agentImage, nil, "tamga-net", mounts); err != nil {
		return fmt.Errorf("create agent container: %w", err)
	}
	if err := s.docker.StartContainer(ctx, containerName); err != nil {
		return fmt.Errorf("start agent container: %w", err)
	}
	slog.Info("agent container started", "container", containerName)
	return nil
}

func (s *AgentService) forwardTask(ctx context.Context, containerName, taskID, message string, projectID int64, dirPath string) error {
	var projectDir string
	if dirPath != "" {
		projectDir = dirPath
	} else {
		projectDir = fmt.Sprintf("%s/projects/%d", s.cfg.DataDir, projectID)
	}

	port := "9000"
	if projectID == 0 && dirPath != "" {
		port = "9001"
	}

	agentURL := fmt.Sprintf("http://%s:%s/chat", containerName, port)

	payload := map[string]string{
		"task_id":     taskID,
		"message":     message,
		"project_dir": projectDir,
	}
	body, _ := json.Marshal(payload)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(agentURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("forward to agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(b))
	}

	for i := 0; i < 300; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		time.Sleep(2 * time.Second)

		var task *domain.AgentTask
		if projectID == 0 && dirPath != "" {
			task, _ = s.GetTaskForDir(context.Background(), "system", taskID)
		} else {
			task, _ = s.GetTask(context.Background(), projectID, taskID)
		}
		if task != nil && (task.Status == domain.AgentTaskStatusCompleted || task.Status == domain.AgentTaskStatusFailed) {
			return nil
		}
	}

	return fmt.Errorf("agent task timed out")
}

func (s *AgentService) pollAgentTask(ctx context.Context, containerName, port string, task *domain.AgentTask) error {
	agentURL := fmt.Sprintf("http://%s:%s/tasks/%s", containerName, port, task.ID)
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(agentURL)
	if err != nil {
		return fmt.Errorf("poll agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent returned %d", resp.StatusCode)
	}

	var agentTask struct {
		Status      string `json:"status"`
		Response    string `json:"response"`
		Diff        string `json:"diff"`
		CompletedAt string `json:"completed_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agentTask); err != nil {
		return fmt.Errorf("decode agent task: %w", err)
	}

	task.Status = domain.AgentTaskStatus(agentTask.Status)
	task.Response = agentTask.Response
	task.Diff = agentTask.Diff
	if agentTask.CompletedAt != "" {
		t, _ := time.Parse(time.RFC3339, agentTask.CompletedAt)
		task.CompletedAt = &t
	}
	s.db.UpdateAgentTask(task)
	return nil
}
