package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/google/uuid"
)

type AgentService struct {
	db          *sqlite.DB
	docker      *dockerclient.Client
	cfg         config.Config
	providerSvc *AgentProviderService

	mu           sync.Mutex
	lastActivity map[string]time.Time
	startOnce    sync.Once
}

func NewAgentService(db *sqlite.DB, docker *dockerclient.Client, cfg config.Config, providerSvc *AgentProviderService) *AgentService {
	return &AgentService{
		db:           db,
		docker:       docker,
		cfg:          cfg,
		providerSvc:  providerSvc,
		lastActivity: make(map[string]time.Time),
	}
}

const agentImage = "tamga-agent"
const idleTimeout = 30 * time.Minute

func (s *AgentService) ensureContainerRunning(ctx context.Context, containerName string, mounts []string) error {
	if s.docker == nil {
		return fmt.Errorf("docker daemon not available")
	}
	if s.docker.ContainerIsRunning(ctx, containerName) {
		return nil
	}
	if s.docker.ContainerExists(ctx, containerName) {
		if err := s.docker.StartContainer(ctx, containerName); err != nil {
			slog.Warn("failed to start existing agent container, recreating", "container", containerName, "error", err)
			s.docker.RemoveContainer(ctx, containerName)
			if _, err := s.docker.CreateContainerOpts(ctx, containerName, agentImage, nil, "tamga-net", mounts); err != nil {
				return fmt.Errorf("recreate agent container: %w", err)
			}
			if err := s.docker.StartContainer(ctx, containerName); err != nil {
				return fmt.Errorf("start recreated agent container: %w", err)
			}
			slog.Info("agent container recreated and started", "container", containerName)
			return nil
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

func (s *AgentService) getOrCreateSession(ctx context.Context, projectID int64, sessionID, message string) (string, error) {
	if sessionID != "" {
		sess, err := s.db.FindAgentSession(sessionID)
		if err == nil {
			sess.UpdatedAt = time.Now()
			s.db.UpdateAgentSession(sess)
			return sess.ID, nil
		}
	}

	name := message
	if len(name) > 60 {
		name = name[:60] + "..."
	}

	id := uuid.New().String()
	now := time.Now()
	sess := &domain.AgentSession{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.CreateAgentSession(sess); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

func (s *AgentService) resolveProviderForProject(ctx context.Context, projectID int64) (*domain.AgentProvider, error) {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	return s.providerSvc.ResolveProvider(safeDeref(project.AgentProviderID))
}

func (s *AgentService) Chat(ctx context.Context, projectID int64, message string, sessionID *string) (*domain.AgentTask, error) {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	if project.Status != domain.ProjectStatusRunning {
		return nil, fmt.Errorf("project is not running")
	}

	sid, err := s.getOrCreateSession(ctx, projectID, safeDeref(sessionID), message)
	if err != nil {
		return nil, fmt.Errorf("resolve session: %w", err)
	}

	taskID := uuid.New().String()
	task := &domain.AgentTask{
		ID:        taskID,
		ProjectID: projectID,
		SessionID: &sid,
		Message:   message,
		Status:    domain.AgentTaskStatusPending,
	}
	if err := s.db.CreateAgentTask(task); err != nil {
		return nil, fmt.Errorf("create agent task: %w", err)
	}

	provider, err := s.resolveProviderForProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
	}

	containerName := fmt.Sprintf("agent-%d", projectID)
	bridge := s.providerSvc.BuildBridge(provider, containerName)

	if provider.Type == domain.ProviderTypeDocker {
		if err := s.ensureContainerRunning(ctx, containerName, nil); err != nil {
			slog.Error("ensure agent container running failed", "container", containerName, "error", err)
		}
		s.trackActivity(containerName)
		s.startIdleWatcher()
	}

	projectDir := fmt.Sprintf("%s/projects/%d", s.cfg.DataDir, projectID)
	go func() {
		if err := bridge.ForwardTask(context.Background(), taskID, message, projectDir); err != nil {
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

	provider, err := s.resolveProviderForProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
	}

	containerName := fmt.Sprintf("agent-%d", projectID)
	bridge := s.providerSvc.BuildBridge(provider, containerName)

	if err := bridge.PollTask(ctx, taskID, task); err != nil {
		slog.Warn("poll agent task error", "task_id", taskID, "error", err)
	}
	s.db.UpdateAgentTask(task)

	return task, nil
}

func (s *AgentService) ListTasks(ctx context.Context, projectID int64) ([]*domain.AgentTask, error) {
	return s.db.ListAgentTasks(projectID)
}

func (s *AgentService) IsAgentRunning(ctx context.Context, projectID int64) bool {
	if s.docker == nil {
		return false
	}
	containerName := fmt.Sprintf("agent-%d", projectID)
	return s.docker.ContainerIsRunning(ctx, containerName)
}

func (s *AgentService) StartAgent(ctx context.Context, projectID int64) error {
	containerName := fmt.Sprintf("agent-%d", projectID)
	return s.ensureContainerRunning(ctx, containerName, nil)
}

func (s *AgentService) StopAgent(ctx context.Context, projectID int64) error {
	if s.docker == nil {
		return nil
	}
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

func safeDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *AgentService) ChatForDir(ctx context.Context, dirID, dirPath, message string, sessionID *string) (*domain.AgentTask, error) {
	if s.docker == nil {
		return nil, fmt.Errorf("docker daemon not available")
	}

	sid, err := s.getOrCreateSession(ctx, 0, safeDeref(sessionID), message)
	if err != nil {
		return nil, fmt.Errorf("resolve session: %w", err)
	}

	taskID := uuid.New().String()

	s.db.CreateAgentTask(&domain.AgentTask{
		ID:        taskID,
		ProjectID: 0,
		SessionID: &sid,
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

	provider, err := s.providerSvc.ResolveProvider("")
	if err != nil {
		return nil, fmt.Errorf("resolve default provider: %w", err)
	}
	bridge := s.providerSvc.BuildBridge(provider, containerName)

	go func() {
		containerDir := fmt.Sprintf("/workspace/%s", dirID)
		if err := bridge.ForwardTask(context.Background(), taskID, message, containerDir); err != nil {
			slog.Error("forward task to dir agent failed", "task_id", taskID, "error", err)
		}
	}()

	return &domain.AgentTask{ID: taskID}, nil
}

func (s *AgentService) CreateSession(ctx context.Context, projectID int64, name string) (*domain.AgentSession, error) {
	id := uuid.New().String()
	now := time.Now()
	sess := &domain.AgentSession{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.CreateAgentSession(sess); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sess, nil
}

func (s *AgentService) ListSessions(ctx context.Context, projectID int64) ([]*domain.AgentSession, error) {
	return s.db.ListAgentSessions(projectID)
}

func (s *AgentService) RenameSession(ctx context.Context, sessionID, name string) error {
	sess, err := s.db.FindAgentSession(sessionID)
	if err != nil {
		return fmt.Errorf("find session: %w", err)
	}
	sess.Name = name
	sess.UpdatedAt = time.Now()
	return s.db.UpdateAgentSession(sess)
}

func (s *AgentService) DeleteSession(ctx context.Context, sessionID string) error {
	return s.db.DeleteAgentSession(sessionID)
}

func (s *AgentService) ListTasksBySession(ctx context.Context, sessionID string) ([]*domain.AgentTask, error) {
	return s.db.ListAgentTasksBySession(sessionID)
}

func (s *AgentService) GetTaskForDir(ctx context.Context, dirID, taskID string) (*domain.AgentTask, error) {
	task, err := s.db.FindAgentTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("find task: %w", err)
	}

	containerName := fmt.Sprintf("agent-%s", dirID)

	provider, err := s.providerSvc.ResolveProvider("")
	if err != nil {
		return nil, fmt.Errorf("resolve default provider: %w", err)
	}
	bridge := s.providerSvc.BuildBridge(provider, containerName)

	if err := bridge.PollTask(ctx, taskID, task); err != nil {
		slog.Warn("poll agent task error", "task_id", taskID, "error", err)
	}
	s.db.UpdateAgentTask(task)

	return task, nil
}

func (s *AgentService) ListTasksForDir(ctx context.Context) ([]*domain.AgentTask, error) {
	return s.db.ListAgentTasks(0)
}

func (s *AgentService) IsAgentRunningForDir(ctx context.Context, dirID string) bool {
	if s.docker == nil {
		return false
	}
	containerName := fmt.Sprintf("agent-%s", dirID)
	return s.docker.ContainerIsRunning(ctx, containerName)
}

func (s *AgentService) StartAgentForDir(ctx context.Context, dirID, dirPath string) error {
	containerName := fmt.Sprintf("agent-%s", dirID)
	mounts := []string{fmt.Sprintf("%s:/workspace/%s", dirPath, dirID)}
	return s.ensureContainerRunning(ctx, containerName, mounts)
}

func (s *AgentService) StopAgentForDir(ctx context.Context, dirID string) error {
	if s.docker == nil {
		return nil
	}
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

func (s *AgentService) UpdateProjectProvider(ctx context.Context, projectID int64, providerID string) error {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	project.AgentProviderID = &providerID
	return s.db.UpdateProject(project)
}
