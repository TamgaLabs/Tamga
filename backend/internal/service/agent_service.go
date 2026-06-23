package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
}

func NewAgentService(db *sqlite.DB, docker *dockerclient.Client, cfg config.Config) *AgentService {
	return &AgentService{db: db, docker: docker, cfg: cfg}
}

const agentImage = "tamga-agent"

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
	if !s.docker.ContainerExists(ctx, containerName) {
		go func() {
			if err := s.startAgentContainer(context.Background(), containerName); err != nil {
				slog.Error("start agent container failed", "container", containerName, "error", err)
			}
		}()
	}

	go func() {
		if err := s.forwardTask(context.Background(), containerName, taskID, message, projectID); err != nil {
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
	agentURL := fmt.Sprintf("http://%s:9000/tasks/%s", containerName, taskID)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(agentURL)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var agentTask struct {
				Status      string `json:"status"`
				Response    string `json:"response"`
				Diff        string `json:"diff"`
				CompletedAt string `json:"completed_at"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&agentTask); err == nil {
				task.Status = domain.AgentTaskStatus(agentTask.Status)
				task.Response = agentTask.Response
				task.Diff = agentTask.Diff
				if agentTask.CompletedAt != "" {
					t, _ := time.Parse(time.RFC3339, agentTask.CompletedAt)
					task.CompletedAt = &t
				}
				s.db.UpdateAgentTask(task)
			}
		}
	}

	return task, nil
}

func (s *AgentService) startAgentContainer(ctx context.Context, containerName string) error {
	if _, err := s.docker.CreateContainer(ctx, containerName, agentImage, nil, "tamga-net"); err != nil {
		return fmt.Errorf("create agent container: %w", err)
	}
	if err := s.docker.StartContainer(ctx, containerName); err != nil {
		return fmt.Errorf("start agent container: %w", err)
	}
	slog.Info("agent container started", "container", containerName)
	return nil
}

func (s *AgentService) forwardTask(ctx context.Context, containerName, taskID, message string, projectID int64) error {
	projectDir := fmt.Sprintf("%s/projects/%d", s.cfg.DataDir, projectID)
	agentURL := fmt.Sprintf("http://%s:9000/chat", containerName)

	payload := map[string]string{
		"task_id":     taskID,
		"message":     message,
		"project_dir": projectDir,
	}
	body, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(agentURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("forward to agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(b))
	}

	// wait for completion with polling
	for i := 0; i < 300; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		time.Sleep(2 * time.Second)

		task, err := s.GetTask(context.Background(), projectID, taskID)
		if err != nil {
			slog.Warn("poll task error", "task_id", taskID, "error", err)
			continue
		}
		if task.Status == domain.AgentTaskStatusCompleted || task.Status == domain.AgentTaskStatusFailed {
			return nil
		}
	}

	return fmt.Errorf("agent task timed out")
}
