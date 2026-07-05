package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/docker/docker/api/types"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// AgentService owns the lifecycle of per-project sandbox containers and the
// shell exec session a terminal WebSocket connection proxies into. It no
// longer knows anything about a specific agent CLI or protocol - it only
// starts/stops containers and opens a shell inside them.
type AgentService struct {
	db          *sqlite.DB
	docker      *dockerclient.Client
	cfg         config.Config
	providerSvc *AgentProviderService
	apiKeySvc   *ApiKeyService

	mu        sync.Mutex
	connCount map[string]int
}

func NewAgentService(db *sqlite.DB, docker *dockerclient.Client, cfg config.Config, providerSvc *AgentProviderService, apiKeySvc *ApiKeyService) *AgentService {
	return &AgentService{
		db:          db,
		docker:      docker,
		cfg:         cfg,
		providerSvc: providerSvc,
		apiKeySvc:   apiKeySvc,
		connCount:   make(map[string]int),
	}
}

const agentImage = "tamga-agent"

func (s *AgentService) ensureContainerRunning(ctx context.Context, containerName string, mounts []string, image string, env []string) error {
	if s.docker == nil {
		return fmt.Errorf("docker daemon not available")
	}
	if image == "" {
		image = agentImage
	}
	if s.docker.ContainerIsRunning(ctx, containerName) {
		return nil
	}
	if s.docker.ContainerExists(ctx, containerName) {
		if err := s.docker.StartContainer(ctx, containerName); err != nil {
			slog.Warn("failed to start existing agent container, recreating", "container", containerName, "error", err)
			s.docker.RemoveContainer(ctx, containerName)
			if _, err := s.docker.CreateContainerOpts(ctx, containerName, image, env, "tamga-net", mounts); err != nil {
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
	if _, err := s.docker.CreateContainerOpts(ctx, containerName, image, env, "tamga-net", mounts); err != nil {
		return fmt.Errorf("create agent container: %w", err)
	}
	if err := s.docker.StartContainer(ctx, containerName); err != nil {
		return fmt.Errorf("start agent container: %w", err)
	}
	slog.Info("agent container created and started", "container", containerName)
	return nil
}

func (s *AgentService) injectApiKeys(env []string) []string {
	if s.apiKeySvc == nil {
		return env
	}
	keyEnv, err := s.apiKeySvc.GetAllAsEnv()
	if err != nil {
		slog.Warn("failed to get api keys for injection", "error", err)
		return env
	}
	for k, v := range keyEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

func (s *AgentService) resolveProviderForProject(ctx context.Context, projectID int64) (*domain.AgentProvider, error) {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	return s.providerSvc.ResolveProvider(safeDeref(project.AgentProviderID))
}

// StartSandbox ensures the project's sandbox container is running and
// registers this caller as an active terminal connection against it, so a
// later ReleaseSandbox call only stops the container once every terminal
// connection for the project has closed. It returns the container name and
// the workspace directory to exec into.
//
// TODO(FEAT-008): this is the natural injection point for per-project git
// credentials once the global git credential store lands - add them to env
// here before the container is created.
func (s *AgentService) StartSandbox(ctx context.Context, projectID int64) (containerName, workDir string, err error) {
	if s.docker == nil {
		return "", "", fmt.Errorf("docker daemon not available")
	}

	provider, err := s.resolveProviderForProject(ctx, projectID)
	if err != nil {
		return "", "", fmt.Errorf("resolve provider: %w", err)
	}

	containerName = fmt.Sprintf("agent-%d", projectID)
	image := provider.Image
	if image == "" {
		image = agentImage
	}

	var env []string
	if provider.Env != "" {
		var envMap map[string]string
		if jerr := json.Unmarshal([]byte(provider.Env), &envMap); jerr == nil {
			for k, v := range envMap {
				env = append(env, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}
	env = s.injectApiKeys(env)

	mounts := []string{fmt.Sprintf("%s/projects/%d:/workspace/%d", s.cfg.DataDir, projectID, projectID)}

	// Hold the lock across the full ensure-container-then-increment sequence
	// so a concurrent StartSandbox/ReleaseSandbox for the same container name
	// can never observe (or act on) a half-finished state - see FEAT-004
	// review notes for the two races this closes.
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureContainerRunning(ctx, containerName, mounts, image, env); err != nil {
		return "", "", err
	}

	s.connCount[containerName]++

	return containerName, fmt.Sprintf("/workspace/%d", projectID), nil
}

// ReleaseSandbox unregisters an active terminal connection for the project.
// Once no terminal connections remain, the sandbox container is stopped and
// removed - this is what makes the sandbox lifecycle ephemeral.
func (s *AgentService) ReleaseSandbox(ctx context.Context, projectID int64) {
	containerName := fmt.Sprintf("agent-%d", projectID)

	// Hold the lock across decrement AND the resulting stop/remove so a
	// StartSandbox that lands in between can't have its (freshly
	// re-incremented) container yanked out from under it - see FEAT-004
	// review notes.
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connCount[containerName]--
	remaining := s.connCount[containerName]
	if remaining <= 0 {
		delete(s.connCount, containerName)
	}

	if remaining > 0 {
		return
	}
	if err := s.StopAgent(ctx, projectID); err != nil {
		slog.Warn("failed to stop sandbox after terminal closed", "container", containerName, "error", err)
	}
}

// OpenShell starts a shell process (PTY) inside the sandbox container and
// returns the exec ID, ready to be attached to.
func (s *AgentService) OpenShell(ctx context.Context, containerName, workDir string) (string, error) {
	if s.docker == nil {
		return "", fmt.Errorf("docker daemon not available")
	}
	return s.docker.ExecCreate(ctx, containerName, []string{"/bin/sh"}, workDir)
}

// AttachShell attaches to a previously created exec session, returning the
// hijacked stdio stream to proxy over the terminal WebSocket.
func (s *AgentService) AttachShell(ctx context.Context, execID string) (types.HijackedResponse, error) {
	if s.docker == nil {
		return types.HijackedResponse{}, fmt.Errorf("docker daemon not available")
	}
	return s.docker.ExecAttach(ctx, execID)
}

// ResizeShell resizes the PTY behind an exec session to match the browser
// terminal's dimensions.
func (s *AgentService) ResizeShell(ctx context.Context, execID string, rows, cols uint) error {
	if s.docker == nil {
		return fmt.Errorf("docker daemon not available")
	}
	return s.docker.ExecResize(ctx, execID, rows, cols)
}

func (s *AgentService) StopAgent(ctx context.Context, projectID int64) error {
	if s.docker == nil {
		return nil
	}
	containerName := fmt.Sprintf("agent-%d", projectID)

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

func (s *AgentService) UpdateProjectProvider(ctx context.Context, projectID int64, providerID string) error {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	project.AgentProviderID = &providerID
	return s.db.UpdateProject(project)
}
