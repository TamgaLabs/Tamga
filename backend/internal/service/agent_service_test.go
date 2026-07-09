package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

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
	resourceLimitSvc := NewResourceLimitService(db)

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

	agentSvc := NewAgentService(db, docker, cfg, whitelistSvc, resourceLimitSvc, nil)
	return agentSvc, docker, project.ID
}

// TestAgentServiceSandboxLifecycle exercises the create-on-connect/
// destroy-on-disconnect refcounted lifecycle against a real Docker daemon:
// two overlapping "terminal connections" share one sandbox container, and
// the container is only stopped/removed once both have released it.
func TestAgentServiceSandboxLifecycle(t *testing.T) {
	agentSvc, docker, projectID := newTestAgentService(t)
	ctx := context.Background()
	containerName := fmt.Sprintf("agent-%d", projectID)

	// First terminal connection creates the sandbox.
	name, workDir, err := agentSvc.StartSandbox(ctx, projectID)
	if err != nil {
		t.Fatalf("start sandbox: %v", err)
	}
	if name != containerName {
		t.Fatalf("expected container name %q, got %q", containerName, name)
	}
	if workDir != fmt.Sprintf("/workspace/%d", projectID) {
		t.Fatalf("unexpected workdir: %q", workDir)
	}
	if !docker.ContainerIsRunning(ctx, containerName) {
		t.Fatal("expected sandbox container to be running after StartSandbox")
	}

	// A second overlapping terminal connection reuses the same container
	// rather than creating a new one, and bumps the refcount.
	if _, _, err := agentSvc.StartSandbox(ctx, projectID); err != nil {
		t.Fatalf("start sandbox (second connection): %v", err)
	}
	if got := agentSvc.connCount[containerName]; got != 2 {
		t.Fatalf("expected connCount 2 after two StartSandbox calls, got %d", got)
	}

	// Exec/attach at the service boundary: open a shell and attach to it.
	execID, err := agentSvc.OpenShell(ctx, containerName, workDir)
	if err != nil {
		t.Fatalf("open shell: %v", err)
	}
	if execID == "" {
		t.Fatal("expected non-empty exec ID")
	}
	hijack, err := agentSvc.AttachShell(ctx, execID)
	if err != nil {
		t.Fatalf("attach shell: %v", err)
	}
	hijack.Close()

	// Releasing one connection must not stop the container - the other
	// connection is still active.
	agentSvc.ReleaseSandbox(ctx, projectID)
	if got := agentSvc.connCount[containerName]; got != 1 {
		t.Fatalf("expected connCount 1 after one ReleaseSandbox, got %d", got)
	}
	if !docker.ContainerIsRunning(ctx, containerName) {
		t.Fatal("expected sandbox container to still be running with one connection left")
	}

	// Releasing the last connection tears the sandbox down.
	agentSvc.ReleaseSandbox(ctx, projectID)
	if _, ok := agentSvc.connCount[containerName]; ok {
		t.Fatal("expected connCount entry to be removed once it reaches zero")
	}

	// Container removal happens synchronously inside ReleaseSandbox, but
	// give the daemon a brief moment to settle before asserting.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && docker.ContainerExists(ctx, containerName) {
		time.Sleep(50 * time.Millisecond)
	}
	if docker.ContainerExists(ctx, containerName) {
		t.Fatal("expected sandbox container to be removed after last ReleaseSandbox")
	}
}
