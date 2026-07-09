package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// AgentService owns the lifecycle of per-project sandbox containers and the
// shell exec session a terminal WebSocket connection proxies into. It no
// longer knows anything about a specific agent CLI or protocol - it only
// starts/stops containers and opens a shell inside them.
//
// Networking (FEAT-006): each project's sandbox gets its own internal
// Docker network (no route to the internet, no route to tamga-net or any
// other project's sandbox). The only way out is through the shared
// egress-proxy container, which is multi-homed onto every active sandbox's
// network and only allows CONNECT/requests to whitelisted domains.
type AgentService struct {
	db            *sqlite.DB
	docker        *dockerclient.Client
	cfg           config.Config
	providerSvc   *AgentProviderService
	apiKeySvc     *ApiKeyService
	whitelistSvc  *WhitelistService
	resourceLimit *ResourceLimitService
	gitCredSvc    *GitCredentialService

	mu        sync.Mutex
	connCount map[string]int
}

func NewAgentService(db *sqlite.DB, docker *dockerclient.Client, cfg config.Config, providerSvc *AgentProviderService, apiKeySvc *ApiKeyService, whitelistSvc *WhitelistService, resourceLimitSvc *ResourceLimitService, gitCredSvc *GitCredentialService) *AgentService {
	return &AgentService{
		db:            db,
		docker:        docker,
		cfg:           cfg,
		providerSvc:   providerSvc,
		apiKeySvc:     apiKeySvc,
		whitelistSvc:  whitelistSvc,
		resourceLimit: resourceLimitSvc,
		gitCredSvc:    gitCredSvc,
		connCount:     make(map[string]int),
	}
}

// defaultSandboxMemoryBytes and defaultSandboxNanoCPUs are the hardcoded
// fallback resource limit applied to a sandbox if the configured default
// can't be loaded (e.g. a transient DB error) - a sandbox must never be
// created with no limit at all, per FEAT-007.
const (
	defaultSandboxMemoryBytes int64 = 1 << 30       // 1 GiB
	defaultSandboxNanoCPUs    int64 = 1_000_000_000 // 1 CPU
)

// sandboxResources returns the CPU/memory limit to apply to a newly
// created sandbox container, per the global default stored in Settings
// (see FEAT-007). Falls back to a hardcoded safe default if the setting
// can't be loaded, so no sandbox is ever created unlimited.
func (s *AgentService) sandboxResources() container.Resources {
	if s.resourceLimit != nil {
		if rl, err := s.resourceLimit.Get(); err == nil {
			return container.Resources{Memory: rl.MemoryBytes, NanoCPUs: rl.NanoCPUs}
		} else {
			slog.Warn("load default sandbox resource limit, using hardcoded fallback", "error", err)
		}
	}
	return container.Resources{Memory: defaultSandboxMemoryBytes, NanoCPUs: defaultSandboxNanoCPUs}
}

const agentImage = "tamga-agent"

// Egress proxy: a single shared, always-on container that is the sole
// route off every sandbox's internal network out to the internet. See
// ensureEgressProxy.
const (
	egressProxyImage = "tamga-egress-proxy"
	egressProxyName  = "tamga-egress-proxy"
	egressProxyPort  = "8888"
)

// agentNetworkName returns the dedicated Docker network name for a
// project's sandbox. Each project gets its own network (rather than
// sharing one) so sandboxes can never reach each other directly - they
// simply aren't on the same network.
func agentNetworkName(projectID int64) string {
	return fmt.Sprintf("agent-net-%d", projectID)
}

// activeAgentNetworks returns the sandbox networks that currently have a
// running container attached, derived from connCount. Caller must hold
// s.mu.
func (s *AgentService) activeAgentNetworks() []string {
	var nets []string
	for name := range s.connCount {
		var projectID int64
		if _, err := fmt.Sscanf(name, "agent-%d", &projectID); err == nil {
			nets = append(nets, agentNetworkName(projectID))
		}
	}
	return nets
}

// ensureEgressProxy makes sure the shared egress-proxy container is
// running with the current whitelist and is attached to every network in
// networks. It recreates the container whenever the whitelist has changed
// since it was last (re)created - this is what makes whitelist edits take
// effect on the next sandbox creation, per FEAT-006.
func (s *AgentService) ensureEgressProxy(ctx context.Context, networks []string) error {
	domains, err := s.whitelistSvc.Domains()
	if err != nil {
		return fmt.Errorf("load egress whitelist: %w", err)
	}
	sort.Strings(domains)
	wantEnv := fmt.Sprintf("ALLOWED_DOMAINS=%s", strings.Join(domains, ","))

	upToDate := false
	if s.docker.ContainerIsRunning(ctx, egressProxyName) {
		currentEnv, err := s.docker.ContainerEnv(ctx, egressProxyName)
		if err != nil {
			slog.Warn("inspect egress proxy env, recreating", "error", err)
		} else {
			for _, e := range currentEnv {
				if e == wantEnv {
					upToDate = true
					break
				}
			}
		}
	}

	if !upToDate {
		if s.docker.ContainerExists(ctx, egressProxyName) {
			s.docker.StopContainerTimeout(ctx, egressProxyName, 2)
			if err := s.docker.RemoveContainer(ctx, egressProxyName); err != nil {
				return fmt.Errorf("remove stale egress proxy: %w", err)
			}
		}
		env := []string{wantEnv, fmt.Sprintf("PORT=%s", egressProxyPort)}
		// "bridge" is Docker's always-present default network - this is
		// the proxy's one and only route to the internet. Per-project
		// sandbox networks are attached below via NetworkConnect.
		if _, err := s.docker.CreateContainerOpts(ctx, egressProxyName, egressProxyImage, env, "bridge", nil, container.Resources{}, true); err != nil {
			return fmt.Errorf("create egress proxy: %w", err)
		}
		if err := s.docker.StartContainer(ctx, egressProxyName); err != nil {
			return fmt.Errorf("start egress proxy: %w", err)
		}
		slog.Info("egress proxy (re)created", "domains", domains)
	}

	for _, netName := range networks {
		if err := s.docker.NetworkConnect(ctx, netName, egressProxyName); err != nil {
			slog.Warn("connect egress proxy to sandbox network", "network", netName, "error", err)
		}
	}
	return nil
}

// egressProxyEnv returns the HTTP(S)_PROXY env vars pointing sandbox
// traffic at the shared egress proxy, reachable by container name on the
// sandbox's own network.
func egressProxyEnv() []string {
	proxyURL := fmt.Sprintf("http://%s:%s", egressProxyName, egressProxyPort)
	return []string{
		fmt.Sprintf("HTTP_PROXY=%s", proxyURL),
		fmt.Sprintf("HTTPS_PROXY=%s", proxyURL),
		fmt.Sprintf("http_proxy=%s", proxyURL),
		fmt.Sprintf("https_proxy=%s", proxyURL),
		"NO_PROXY=localhost,127.0.0.1",
		"no_proxy=localhost,127.0.0.1",
	}
}

func (s *AgentService) ensureContainerRunning(ctx context.Context, containerName string, mounts []string, image string, env []string, network string, resources container.Resources) error {
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
			if _, err := s.docker.CreateContainerOpts(ctx, containerName, image, env, network, mounts, resources, true); err != nil {
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
	if _, err := s.docker.CreateContainerOpts(ctx, containerName, image, env, network, mounts, resources, true); err != nil {
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

// injectGitCredential appends the env vars that configure git inside the
// sandbox to authenticate as the global git credential (see FEAT-008), so
// `git commit`/`push` works from the terminal without manual auth. No-op
// (env unchanged) if no credential is configured or it can't be loaded.
func (s *AgentService) injectGitCredential(env []string) []string {
	if s.gitCredSvc == nil {
		return env
	}
	gitEnv, err := s.gitCredSvc.SandboxEnv()
	if err != nil {
		slog.Warn("failed to get git credential for sandbox injection", "error", err)
		return env
	}
	return append(env, gitEnv...)
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
	env = s.injectGitCredential(env)
	env = append(env, egressProxyEnv()...)

	// Validate HostDataDir is set and absolute before constructing mount string.
	// This ensures the bind-mount source passed to the Docker daemon is correct.
	if s.cfg.HostDataDir == "" || !filepath.IsAbs(s.cfg.HostDataDir) {
		return "", "", fmt.Errorf("HOST_DATA_DIR must be set to an absolute host path (got: %q); see .env.example or set HOST_DATA_DIR explicitly", s.cfg.HostDataDir)
	}

	mounts := []string{fmt.Sprintf("%s/projects/%d:/workspace/%d", s.cfg.HostDataDir, projectID, projectID)}
	network := agentNetworkName(projectID)

	// Hold the lock across the full ensure-container-then-increment sequence
	// so a concurrent StartSandbox/ReleaseSandbox for the same container name
	// can never observe (or act on) a half-finished state - see FEAT-004
	// review notes for the two races this closes.
	s.mu.Lock()
	defer s.mu.Unlock()

	// FEAT-006: this project's sandbox gets its own internal network,
	// isolated from tamga-net, project containers and every other
	// project's sandbox. Outbound internet only exists via the shared
	// egress proxy, which we (re)connect to every currently-active
	// sandbox network plus this one so whitelist edits and new sandboxes
	// both land on an up-to-date proxy.
	if err := s.docker.EnsureNetwork(ctx, network, true); err != nil {
		return "", "", fmt.Errorf("ensure sandbox network: %w", err)
	}
	networks := append(s.activeAgentNetworks(), network)
	if err := s.ensureEgressProxy(ctx, networks); err != nil {
		return "", "", fmt.Errorf("ensure egress proxy: %w", err)
	}

	if err := s.ensureContainerRunning(ctx, containerName, mounts, image, env, network, s.sandboxResources()); err != nil {
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
	network := agentNetworkName(projectID)

	if !s.docker.ContainerExists(ctx, containerName) {
		return nil
	}
	if err := s.docker.StopContainerTimeout(ctx, containerName, 2); err != nil {
		return fmt.Errorf("stop agent: %w", err)
	}
	if err := s.docker.RemoveContainer(ctx, containerName); err != nil {
		return fmt.Errorf("remove agent: %w", err)
	}
	slog.Info("agent container stopped", "container", containerName)

	// The sandbox was the only other member of its network besides the
	// egress proxy - tear both down now that it's gone, so we don't
	// accumulate one internal network per project ever created.
	if err := s.docker.NetworkDisconnect(ctx, network, egressProxyName); err != nil {
		slog.Warn("disconnect egress proxy from sandbox network", "network", network, "error", err)
	}
	if err := s.docker.NetworkRemove(ctx, network); err != nil {
		slog.Warn("remove sandbox network", "network", network, "error", err)
	}
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
