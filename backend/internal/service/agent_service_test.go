package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// This file is a deliberate exception to FEAT-021's move of tests into
// internal/tests/: TestAgentServiceSessionCapEnforcement populates
// AgentService's unexported sessions registry and builds TerminalSession
// values via their unexported ring/done fields directly, and reaches for
// the unexported maxSessionsPerProject - on purpose, so the cap check can
// be verified without going through a real (Docker-dependent) session
// create for every one of the maxSessionsPerProject+1 sessions it needs.
// There is no exported way to seed the registry like this, so it stays
// colocated.

// newTestAgentService builds an AgentService backed by a real throwaway
// SQLite DB and a real Docker client talking to the daemon available in
// this environment (see Proposed Solution: AgentService's docker field is
// a concrete *dockerclient.Client, not an interface, so there is nothing
// to fake without introducing a speculative abstraction the production
// code doesn't otherwise need - a real daemon is used instead). The test
// skips itself if no daemon is reachable, so it degrades gracefully
// somewhere that assumption doesn't hold (e.g. plain CI without
// docker-in-docker).
func newTestAgentService(t *testing.T) (*AgentService, *dockerclient.Client, int64) {
	t.Helper()

	docker, err := dockerclient.New()
	if err != nil {
		t.Skipf("docker client not available: %v", err)
	}
	if _, err := docker.DockerInfo(context.Background()); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}

	dbPath := "/tmp/test_agent_service_" + t.Name() + ".db"
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
		Name:       "sandbox-test",
		SourceType: domain.SourceTypeRemote,
		Branch:     "main",
		Status:     domain.ProjectStatusRunning,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	whitelistSvc := NewWhitelistService(db)
	egressSvc := NewEgressService(db)
	resourceLimitSvc := NewResourceLimitService(db)
	idleTimeoutSvc := NewIdleTimeoutService(db)

	hostDataDir := t.TempDir()
	cfg := config.Config{HostDataDir: hostDataDir}

	containerName := fmt.Sprintf("agent-%d", project.ID)
	networkName := agentNetworkName(project.ID)
	t.Cleanup(func() {
		ctx := context.Background()
		docker.StopContainer(ctx, containerName)
		docker.RemoveContainer(ctx, containerName)
		docker.NetworkDisconnect(ctx, networkName, egressProxyName)
		docker.NetworkRemove(ctx, networkName)

		// The sandbox container runs as root and leaves root-owned files
		// under the bind-mounted workspace dir, which the (non-root) test
		// process can't remove itself. Clean it up from inside a
		// throwaway container - as root - before t.TempDir()'s own
		// cleanup tries (and would otherwise fail) to remove it.
		cleanup := exec.Command("docker", "run", "--rm", "-v", hostDataDir+":/cleanup", "alpine:3.21", "sh", "-c", "rm -rf /cleanup/*")
		if out, err := cleanup.CombinedOutput(); err != nil {
			t.Logf("best-effort workspace cleanup failed: %v: %s", err, out)
		}
	})

	agentSvc := NewAgentService(db, docker, cfg, whitelistSvc, egressSvc, resourceLimitSvc, nil, idleTimeoutSvc)
	return agentSvc, docker, project.ID
}

// TestAgentServiceSessionCapEnforcement verifies cap checking by directly
// manipulating the registry (without Docker interaction). Full Docker
// integration testing of session lifecycle (create/attach/detach/terminate
// mechanics against a real container) is the tester's job, not unit tests.
func TestAgentServiceSessionCapEnforcement(t *testing.T) {
	agentSvc, _, projectID := newTestAgentService(t)

	// Manually populate the registry to the cap - no Docker needed.
	for i := 0; i < maxSessionsPerProject; i++ {
		sess := &TerminalSession{
			ID:        fmt.Sprintf("fake-session-%d", i),
			ProjectID: projectID,
			ring:      newRingBuffer(1024),
			done:      make(chan struct{}),
		}
		agentSvc.sessions.add(projectID, sess)
	}

	if got := agentSvc.sessions.count(projectID); got != maxSessionsPerProject {
		t.Fatalf("expected registry count %d, got %d", maxSessionsPerProject, got)
	}

	// Attempting to create when at cap should be rejected *before* trying
	// to touch Docker.
	ctx := context.Background()
	_, err := agentSvc.CreateSession(ctx, projectID)
	if !errors.Is(err, ErrSessionCapExceeded) {
		t.Fatalf("expected ErrSessionCapExceeded, got %v", err)
	}

	// A different project is unaffected by the cap on projectID.
	otherProjectID := projectID + 1000
	_, err = agentSvc.CreateSession(ctx, otherProjectID)
	if errors.Is(err, ErrSessionCapExceeded) {
		t.Fatalf("cap should not apply to different project, got %v", err)
	}
	// (The create will fail on Docker setup, but it will fail with a
	// different error, not ErrSessionCapExceeded.)
}
