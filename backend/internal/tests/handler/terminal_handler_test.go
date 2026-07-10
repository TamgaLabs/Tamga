package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/handler"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// newTestTerminalHandler builds a TerminalHandler backed by a real
// AgentService talking to whatever Docker daemon is reachable in this
// environment (mirroring service.newTestAgentService's rationale: AgentService
// wraps a concrete *dockerclient.Client, so there's no seam to fake it
// without adding a speculative interface the production code doesn't
// otherwise need). Skips itself when no daemon is reachable.
func newTestTerminalHandler(t *testing.T) (*handler.TerminalHandler, *service.AgentService, *dockerclient.Client, int64) {
	t.Helper()

	docker, err := dockerclient.New()
	if err != nil {
		t.Skipf("docker client not available: %v", err)
	}
	if _, err := docker.DockerInfo(context.Background()); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}

	dbPath := "/tmp/test_terminal_handler_" + t.Name() + ".db"
	os.Remove(dbPath)
	t.Cleanup(func() {
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	project := &domain.Project{
		Name:       "terminal-handler-test",
		SourceType: domain.SourceTypeRemote,
		Branch:     "main",
		Status:     domain.ProjectStatusRunning,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	whitelistSvc := service.NewWhitelistService(db)
	egressSvc := service.NewEgressService(db)
	resourceLimitSvc := service.NewResourceLimitService(db)
	idleTimeoutSvc := service.NewIdleTimeoutService(db)

	hostDataDir := t.TempDir()
	cfg := config.Config{HostDataDir: hostDataDir}

	containerName := fmt.Sprintf("agent-%d", project.ID)
	networkName := "agent-net-" + fmt.Sprint(project.ID)
	t.Cleanup(func() {
		ctx := context.Background()
		docker.StopContainer(ctx, containerName)
		docker.RemoveContainer(ctx, containerName)
		docker.NetworkRemove(ctx, networkName)

		// The sandbox container runs as root and leaves root-owned files
		// under the bind-mounted workspace dir - clean it up from inside a
		// throwaway container before t.TempDir()'s own cleanup tries (and
		// would otherwise fail) to remove it.
		cleanup := exec.Command("docker", "run", "--rm", "-v", hostDataDir+":/cleanup", "alpine:3.21", "sh", "-c", "rm -rf /cleanup/*")
		if out, err := cleanup.CombinedOutput(); err != nil {
			t.Logf("best-effort workspace cleanup failed: %v: %s", err, out)
		}
	})

	agentSvc := service.NewAgentService(db, docker, cfg, whitelistSvc, egressSvc, resourceLimitSvc, nil, idleTimeoutSvc)
	return handler.NewTerminalHandler(agentSvc), agentSvc, docker, project.ID
}

func terminalRouter(h *handler.TerminalHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/projects/{id}/agent/terminal", h.Serve)
	r.Get("/projects/{id}/agent/sessions", h.ListSessions)
	r.Delete("/projects/{id}/agent/sessions/{sessionId}", h.TerminateSession)
	return r
}

// TestTerminalHandler_FailedUpgradeDoesNotOrphanSession is the BUG-027
// regression test: a request to the terminal endpoint that never completes
// the WebSocket handshake (here, a plain GET with none of the
// Connection/Upgrade headers gorilla/websocket requires, so
// terminalUpgrader.Upgrade fails deterministically) must not leave a
// session behind in the registry, and must not keep the sandbox running.
func TestTerminalHandler_FailedUpgradeDoesNotOrphanSession(t *testing.T) {
	h, agentSvc, docker, projectID := newTestTerminalHandler(t)
	router := terminalRouter(h)
	srv := httptest.NewServer(router)
	defer srv.Close()

	// A plain HTTP GET (no Upgrade/Connection headers) hits the create
	// branch of Serve (CreateSession succeeds, starting the sandbox and
	// the shell), then fails terminalUpgrader.Upgrade - exactly the
	// "aborted handshake" scenario from the bug report.
	resp, err := http.Get(srv.URL + fmt.Sprintf("/projects/%d/agent/terminal", projectID))
	if err != nil {
		t.Fatalf("GET terminal endpoint: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("expected the upgrade to fail (plain GET, no upgrade headers), got 101")
	}

	sessions := agentSvc.ListSessions(projectID)
	if len(sessions) != 0 {
		t.Fatalf("expected no orphaned sessions after a failed upgrade, got %d: %+v", len(sessions), sessions)
	}

	containerName := fmt.Sprintf("agent-%d", projectID)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !docker.ContainerIsRunning(context.Background(), containerName) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("expected sandbox container %q to be stopped after its only session's failed upgrade was cleaned up", containerName)
}

// TestTerminalHandler_AbortLoopDoesNotExhaustCap exercises the abort path
// maxSessionsPerProject+5 times in a row: if cleanup were broken, this
// would leave orphans piling up until legitimate creates start 429ing even
// though every "session" is actually unusable. With the fix, each abort is
// cleaned up immediately, so the registry never grows.
func TestTerminalHandler_AbortLoopDoesNotExhaustCap(t *testing.T) {
	h, agentSvc, _, projectID := newTestTerminalHandler(t)
	router := terminalRouter(h)
	srv := httptest.NewServer(router)
	defer srv.Close()

	const attempts = 15 // > maxSessionsPerProject (10)
	for i := 0; i < attempts; i++ {
		resp, err := http.Get(srv.URL + fmt.Sprintf("/projects/%d/agent/terminal", projectID))
		if err != nil {
			t.Fatalf("attempt %d: GET terminal endpoint: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("attempt %d: got 429 (cap exhausted by orphans) - cleanup regressed", i)
		}
	}

	sessions := agentSvc.ListSessions(projectID)
	if len(sessions) != 0 {
		t.Fatalf("expected no orphaned sessions after %d aborted upgrades, got %d", attempts, len(sessions))
	}
}
