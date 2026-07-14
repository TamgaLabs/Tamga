package service

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	whitelistSvc  *WhitelistService
	egressSvc     *EgressService
	resourceLimit *ResourceLimitService
	gitCredSvc    *GitCredentialService
	idleTimeout   *IdleTimeoutService

	sessions *sessionRegistry
}

func NewAgentService(db *sqlite.DB, docker *dockerclient.Client, cfg config.Config, whitelistSvc *WhitelistService, egressSvc *EgressService, resourceLimitSvc *ResourceLimitService, gitCredSvc *GitCredentialService, idleTimeoutSvc *IdleTimeoutService) *AgentService {
	s := &AgentService{
		db:            db,
		docker:        docker,
		cfg:           cfg,
		whitelistSvc:  whitelistSvc,
		egressSvc:     egressSvc,
		resourceLimit: resourceLimitSvc,
		gitCredSvc:    gitCredSvc,
		idleTimeout:   idleTimeoutSvc,
		sessions:      newSessionRegistry(),
	}
	s.startIdleSweep()
	return s
}

// idleSweepInterval is how often the background sweep (see startIdleSweep)
// checks detached sessions against the configured idle timeout (FEAT-022).
const idleSweepInterval = 60 * time.Second

// startIdleSweep launches the long-lived background goroutine that
// periodically terminates detached terminal sessions that have exceeded
// the configured idle timeout. It is started once, at construction, and
// runs for the lifetime of the process - there is no graceful-shutdown
// plumbing for services in cmd/api/main.go (only the HTTP server itself is
// shut down), so a plain unstoppable goroutine is the KISS choice here; it
// exits when the process does. A nil idleTimeout (e.g. in tests that don't
// wire one) disables the sweep entirely rather than panicking.
func (s *AgentService) startIdleSweep() {
	if s.idleTimeout == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(idleSweepInterval)
		defer ticker.Stop()
		for range ticker.C {
			s.sweepIdleSessions(time.Now())
		}
	}()
}

// idleSessions returns the sessions in `sessions` that are currently
// detached (see TerminalSession.IdleSince) and have been so for at least
// `timeout`, as of `now`. A timeout <= 0 means Never - nothing is ever
// selected. Pure/stateless on purpose so it can be unit tested without a
// running ticker or a Docker daemon (see FEAT-022).
func idleSessions(sessions []*TerminalSession, timeout time.Duration, now time.Time) []*TerminalSession {
	if timeout <= 0 {
		return nil
	}
	var out []*TerminalSession
	for _, sess := range sessions {
		idleSince, detached := sess.IdleSince()
		if !detached {
			continue
		}
		if now.Sub(idleSince) >= timeout {
			out = append(out, sess)
		}
	}
	return out
}

// sweepIdleSessions terminates every currently-detached session that has
// exceeded the configured idle timeout, via the exact same TerminateSession
// path an explicit terminate uses - so last-session-stops-sandbox semantics
// hold here too. A Never (0) timeout is a no-op. Errors terminating an
// individual session are logged and don't stop the sweep from considering
// the rest.
func (s *AgentService) sweepIdleSessions(now time.Time) {
	settings, err := s.idleTimeout.Get()
	if err != nil {
		slog.Warn("idle sweep: load idle timeout setting", "error", err)
		return
	}
	timeout := time.Duration(settings.TimeoutSeconds) * time.Second

	for _, sess := range idleSessions(s.sessions.all(), timeout, now) {
		if err := s.TerminateSession(context.Background(), sess.ProjectID, sess.ID); err != nil {
			slog.Warn("idle sweep: terminate session failed", "project_id", sess.ProjectID, "session_id", sess.ID, "error", err)
		}
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

// ensureEgressProxy makes sure the shared egress-proxy container is
// running with the current egress mode + list(s) and is attached to every
// network in networks. It recreates the container whenever the wanted env
// has changed since it was last (re)created - this is what makes mode
// switches and whitelist/blacklist edits take effect on the next sandbox
// creation, per FEAT-006/FEAT-016.
func (s *AgentService) ensureEgressProxy(ctx context.Context, networks []string) error {
	mode, err := s.egressSvc.GetMode()
	if err != nil {
		return fmt.Errorf("load egress mode: %w", err)
	}

	whitelistDomains, err := s.whitelistSvc.Domains()
	if err != nil {
		return fmt.Errorf("load egress whitelist: %w", err)
	}
	sort.Strings(whitelistDomains)

	wantEnv := []string{
		fmt.Sprintf("MODE=%s", mode),
		fmt.Sprintf("ALLOWED_DOMAINS=%s", strings.Join(whitelistDomains, ",")),
	}

	if mode == domain.EgressModeBlacklist {
		blacklistDomains, err := s.egressSvc.BlacklistDomains()
		if err != nil {
			return fmt.Errorf("load egress blacklist: %w", err)
		}
		sort.Strings(blacklistDomains)
		wantEnv = append(wantEnv, fmt.Sprintf("DENIED_DOMAINS=%s", strings.Join(blacklistDomains, ",")))
	}

	upToDate := false
	if s.docker.ContainerIsRunning(ctx, egressProxyName) {
		currentEnv, err := s.docker.ContainerEnv(ctx, egressProxyName)
		if err != nil {
			slog.Warn("inspect egress proxy env, recreating", "error", err)
		} else {
			upToDate = envContainsAll(currentEnv, wantEnv)
		}
	}

	if !upToDate {
		if s.docker.ContainerExists(ctx, egressProxyName) {
			s.docker.StopContainerTimeout(ctx, egressProxyName, 2)
			if err := s.docker.RemoveContainer(ctx, egressProxyName); err != nil {
				return fmt.Errorf("remove stale egress proxy: %w", err)
			}
		}
		env := append(append([]string{}, wantEnv...), fmt.Sprintf("PORT=%s", egressProxyPort))
		// "bridge" is Docker's always-present default network - this is
		// the proxy's one and only route to the internet. Per-project
		// sandbox networks are attached below via NetworkConnect.
		if _, err := s.docker.CreateContainerOpts(ctx, egressProxyName, egressProxyImage, env, "bridge", nil, container.Resources{}, true, nil); err != nil {
			return fmt.Errorf("create egress proxy: %w", err)
		}
		if err := s.docker.StartContainer(ctx, egressProxyName); err != nil {
			return fmt.Errorf("start egress proxy: %w", err)
		}
		slog.Info("egress proxy (re)created", "mode", mode, "whitelist", whitelistDomains)
	}

	for _, netName := range networks {
		if err := s.docker.NetworkConnect(ctx, netName, egressProxyName, nil); err != nil {
			slog.Warn("connect egress proxy to sandbox network", "network", netName, "error", err)
		}
	}
	return nil
}

// envContainsAll reports whether every entry of want is present in current
// (a subset check) - used because current may contain unrelated env vars
// (e.g. PORT) that aren't part of the diff.
func envContainsAll(current, want []string) bool {
	set := make(map[string]bool, len(current))
	for _, e := range current {
		set[e] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
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
			if _, err := s.docker.CreateContainerOpts(ctx, containerName, image, env, network, mounts, resources, true, nil); err != nil {
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
	if _, err := s.docker.CreateContainerOpts(ctx, containerName, image, env, network, mounts, resources, true, nil); err != nil {
		return fmt.Errorf("create agent container: %w", err)
	}
	if err := s.docker.StartContainer(ctx, containerName); err != nil {
		return fmt.Errorf("start agent container: %w", err)
	}
	slog.Info("agent container created and started", "container", containerName)
	return nil
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

// ensureSandbox makes sure the project's sandbox container is running,
// (re)creating its dedicated network and the shared egress proxy as
// needed, and returns the container name and the workspace directory to
// exec into. Callers must hold s.sessions.projectLock(projectID).
func (s *AgentService) ensureSandbox(ctx context.Context, projectID int64) (containerName, workDir string, err error) {
	if s.docker == nil {
		return "", "", fmt.Errorf("docker daemon not available")
	}

	containerName = fmt.Sprintf("agent-%d", projectID)

	var env []string
	env = s.injectGitCredential(env)
	env = append(env, egressProxyEnv()...)

	// Validate HostDataDir is set and absolute before constructing mount string.
	// This ensures the bind-mount source passed to the Docker daemon is correct.
	if s.cfg.HostDataDir == "" || !filepath.IsAbs(s.cfg.HostDataDir) {
		return "", "", fmt.Errorf("HOST_DATA_DIR must be set to an absolute host path (got: %q); see .env.example or set HOST_DATA_DIR explicitly", s.cfg.HostDataDir)
	}

	mounts := []string{fmt.Sprintf("%s/projects/%d:/workspace/%d", s.cfg.HostDataDir, projectID, projectID)}
	network := agentNetworkName(projectID)

	// FEAT-006: this project's sandbox gets its own internal network,
	// isolated from tamga-net, project containers and every other
	// project's sandbox. Outbound internet only exists via the shared
	// egress proxy, which we (re)connect to every currently-active
	// sandbox network plus this one so whitelist edits and new sandboxes
	// both land on an up-to-date proxy.
	if err := s.docker.EnsureNetwork(ctx, network, true); err != nil {
		return "", "", fmt.Errorf("ensure sandbox network: %w", err)
	}
	networks := append(s.sessions.activeNetworks(), network)
	if err := s.ensureEgressProxy(ctx, networks); err != nil {
		return "", "", fmt.Errorf("ensure egress proxy: %w", err)
	}

	if err := s.ensureContainerRunning(ctx, containerName, mounts, agentImage, env, network, s.sandboxResources()); err != nil {
		return "", "", err
	}

	return containerName, fmt.Sprintf("/workspace/%d", projectID), nil
}

// terminalSessionPidFile is where the wrapper command run by execBash
// records the bash process's own PID, inside the container's own
// filesystem/PID namespace - see execBash and killSessionProcess for why.
func terminalSessionPidFile(sessionID string) string {
	return fmt.Sprintf("/tmp/.tamga-session-%s.pid", sessionID)
}

// execBash starts the session's shell process. It wraps the actual
// `/bin/bash -i` invocation in a tiny `echo $$ > pidfile; exec /bin/bash -i` so
// the shell's own PID - as seen inside the *container's* PID namespace - is
// recorded to a file before `exec` replaces the process image with bash
// (keeping the same PID). See killSessionProcess for why this is needed:
// Docker's Exec API has no "kill this exec" call, and ContainerExecInspect's
// Pid is the PID as seen by the Docker daemon's own (host) namespace, not
// the container's - not something a second `docker exec` into the same
// container could use to signal it. Capturing the PID from inside the
// container's own namespace sidesteps that mismatch entirely.
func (s *AgentService) execBash(ctx context.Context, containerName, workDir, sessionID string) (string, error) {
	pidFile := terminalSessionPidFile(sessionID)
	cmd := []string{"/bin/bash", "-c", fmt.Sprintf("echo $$ > %s; exec /bin/bash -i", pidFile)}
	return s.docker.ExecCreate(ctx, containerName, cmd, workDir)
}

// killSessionProcess terminates one session's bash process without
// touching any other session sharing the same container, by reading back
// the PID execBash recorded and signaling it from a second, short-lived
// exec into the same container (so the kill runs in the same PID
// namespace as the PID it's targeting).
//
// Note: The command does rm -f unconditionally after the kill attempt.
// This ensures that even if kill somehow fails or becomes a no-op (very
// unlikely given valid PIDs in the container's namespace), a retry won't
// get stuck looking for a stale pidfile.
//
// PID-FILE RACE FIX (BUG-027 cleanup path): execBash's "echo $$ > pidfile;
// exec /bin/bash" wrapper writes the pidfile asynchronously, just after
// ExecAttach starts it - there's a brief window right after CreateSession
// returns where the pidfile doesn't exist yet. Session termination used to
// assume enough real-world time (at least a WebSocket round trip) always
// elapsed before anyone tried to kill a session, so this window never
// mattered in practice. BUG-027's terminal_handler.go now calls
// TerminateSession on a just-created session essentially immediately when
// its WebSocket upgrade fails, which hits this window reliably: without a
// wait, `cat pidfile` fails, `kill -9` gets no PID and is a silent no-op,
// and the bash process is never actually killed. The loop below polls (up
// to 2s, well inside ExecRun's own 5s timeout) for the pidfile to appear
// before attempting the kill.
func (s *AgentService) killSessionProcess(ctx context.Context, sess *TerminalSession) error {
	pidFile := terminalSessionPidFile(sess.ID)
	cmd := []string{"/bin/sh", "-c", fmt.Sprintf(
		"i=0; while [ ! -f %s ] && [ $i -lt 20 ]; do sleep 0.1; i=$((i+1)); done; kill -9 $(cat %s 2>/dev/null) 2>/dev/null; rm -f %s",
		pidFile, pidFile, pidFile,
	)}
	return s.docker.ExecRun(ctx, sess.ContainerName, cmd)
}

// CreateSession ensures the project's sandbox is running, starts a new
// bash session inside it and registers it in the project's session
// registry. It enforces maxSessionsPerProject, returning
// ErrSessionCapExceeded rather than silently creating an 11th session.
func (s *AgentService) CreateSession(ctx context.Context, projectID int64) (*TerminalSession, error) {
	if s.docker == nil {
		return nil, fmt.Errorf("docker daemon not available")
	}

	// Per-project lock: serializes this project's sandbox
	// ensure/session-count-and-insert against this project's own
	// terminate/session-end cleanup, without blocking any other
	// project's session activity (see sessionRegistry.projectLock).
	lock := s.sessions.projectLock(projectID)
	lock.Lock()
	defer lock.Unlock()

	if s.sessions.count(projectID) >= maxSessionsPerProject {
		return nil, ErrSessionCapExceeded
	}

	containerName, workDir, err := s.ensureSandbox(ctx, projectID)
	if err != nil {
		return nil, err
	}

	sessionID, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	execID, err := s.execBash(ctx, containerName, workDir, sessionID)
	if err != nil {
		return nil, fmt.Errorf("open shell: %w", err)
	}
	hijacked, err := s.docker.ExecAttach(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("attach shell: %w", err)
	}

	now := time.Now()
	sess := &TerminalSession{
		ID:            sessionID,
		ProjectID:     projectID,
		ContainerName: containerName,
		execID:        execID,
		hijacked:      hijacked,
		ring:          newRingBuffer(terminalRingBufferSize),
		CreatedAt:     now,
		done:          make(chan struct{}),
		// A session starts out detached - CreateSession returns before any
		// WebSocket has attached - so its idle clock (FEAT-022) starts
		// running immediately.
		lastDetachAt: now,
	}
	s.sessions.add(projectID, sess)
	go sess.run(s)

	return sess, nil
}

// GetSession looks up a live session by project and session id.
func (s *AgentService) GetSession(projectID int64, sessionID string) (*TerminalSession, bool) {
	return s.sessions.get(projectID, sessionID)
}

// ListSessions returns every live session for a project, oldest first.
func (s *AgentService) ListSessions(projectID int64) []SessionInfo {
	sessions := s.sessions.list(projectID)
	infos := make([]SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		infos = append(infos, SessionInfo{
			ID:        sess.ID,
			CreatedAt: sess.CreatedAt,
			Connected: sess.Connected(),
		})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].CreatedAt.Before(infos[j].CreatedAt) })
	return infos
}

// TerminateSession explicitly ends one session: it kills the session's
// bash process (see killSessionProcess) and waits (briefly) for the
// session's own reader goroutine to notice the process exited and finish
// its cleanup - which deregisters the session and, if it was the
// project's last one, stops the sandbox (see endSession). Returns
// ErrSessionNotFound if no such session exists. Returns an error if the
// bash process fails to exit within the timeout (the kill didn't work).
func (s *AgentService) TerminateSession(ctx context.Context, projectID int64, sessionID string) error {
	sess, ok := s.sessions.get(projectID, sessionID)
	if !ok {
		return ErrSessionNotFound
	}

	if err := s.killSessionProcess(ctx, sess); err != nil {
		return fmt.Errorf("terminate session: %w", err)
	}

	select {
	case <-sess.done:
		return nil
	case <-time.After(5 * time.Second):
		// If the session doesn't end after sending kill, it means the kill
		// failed - the bash process is still alive. This is a real error.
		return fmt.Errorf("session did not terminate after kill signal (bash process may still be running)")
	}
}

// endSession is run exactly once per session, by that session's own run
// goroutine after its shell process has exited (naturally, or via
// TerminateSession's kill). It deregisters the session and, if this was
// the project's last remaining session, stops the sandbox - the
// "auto-stop when the last session ends" behavior. Per-project locking
// here (rather than a single service-wide lock) means this can run for one
// project while another project's CreateSession/TerminateSession proceeds
// unblocked.
func (s *AgentService) endSession(sess *TerminalSession) {
	lock := s.sessions.projectLock(sess.ProjectID)
	lock.Lock()
	defer lock.Unlock()

	s.sessions.remove(sess.ProjectID, sess.ID)

	if s.sessions.count(sess.ProjectID) == 0 {
		if err := s.StopAgent(context.Background(), sess.ProjectID); err != nil {
			slog.Warn("failed to stop sandbox after last terminal session ended", "project_id", sess.ProjectID, "error", err)
		}
	}
}

// ResizeShell resizes the PTY behind a session's shell process to match
// the browser terminal's dimensions.
func (s *AgentService) ResizeShell(ctx context.Context, sess *TerminalSession, rows, cols uint) error {
	if s.docker == nil {
		return fmt.Errorf("docker daemon not available")
	}
	return s.docker.ExecResize(ctx, sess.execID, rows, cols)
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
