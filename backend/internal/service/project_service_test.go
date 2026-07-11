package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/traefik"
)

// This file is a deliberate exception to FEAT-021's move of tests into
// internal/tests/: TestProjectServiceCloneRepo calls cloneRepo, an
// unexported method, directly. deploy() (the only exported path that
// reaches cloneRepo) calls requireDocker first and bails out before ever
// getting there whenever docker is nil, which is how every other test in
// this package builds ProjectService (no Docker dependency) - so there is
// no black-box way to exercise the clone step without a live Docker
// daemon. The rest of ProjectService's CRUD behavior is covered black-box
// in internal/tests/service/project_service_test.go.

// newTestProjectService builds a ProjectService with a real throwaway
// SQLite DB, no Docker client (docker is nil, matching how the service
// behaves when Docker isn't available - deploy() bails out early via
// requireDocker) and a Traefik client pointed at a throwaway temp
// directory (RemoveRoute/AddRoute are only invoked when a project has a
// container, which the tests below avoid).
func newTestProjectService(t *testing.T) (*ProjectService, config.Config) {
	t.Helper()
	dbPath := "/tmp/test_project_service_" + t.Name() + ".db"
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

	cfg := config.Config{DataDir: t.TempDir()}
	traefikClient := traefik.New(t.TempDir())
	gitCred := NewGitCredentialService(db, "test-jwt-secret")

	return NewProjectService(db, nil, traefikClient, cfg, gitCred), cfg
}

// TestProjectServiceCloneRepo exercises the git-clone-on-create path
// directly against a local bare repository used as a file:// remote, so
// the test is self-contained and needs no network access (mirrors how
// git_credential_service_test.go avoids needing a live GitHub/GitLab
// server - see Proposed Solution).
func TestProjectServiceCloneRepo(t *testing.T) {
	svc, _ := newTestProjectService(t)

	bareRepoDir := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, "", "init", "--bare", bareRepoDir)

	seedDir := t.TempDir()
	runGit(t, seedDir, "init")
	runGit(t, seedDir, "checkout", "-b", "main")
	runGit(t, seedDir, "config", "user.email", "test@tamga.local")
	runGit(t, seedDir, "config", "user.name", "Tamga Test")
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("hello from clone test\n"), 0644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}
	runGit(t, seedDir, "add", "README.md")
	runGit(t, seedDir, "commit", "-m", "initial commit")
	runGit(t, seedDir, "remote", "add", "origin", bareRepoDir)
	runGit(t, seedDir, "push", "origin", "main")

	workDir := filepath.Join(t.TempDir(), "workdir")
	if err := svc.cloneRepo(context.Background(), "file://"+bareRepoDir, "main", workDir); err != nil {
		t.Fatalf("cloneRepo: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workDir, "README.md"))
	if err != nil {
		t.Fatalf("read cloned file: %v", err)
	}
	if string(content) != "hello from clone test\n" {
		t.Fatalf("unexpected cloned content: %q", content)
	}
}

// TestProjectServiceDeployStackServiceNameAlias is TEST-014's rework
// close-out: exercises deployStack itself (not just the docker client
// primitive it's built on) with a real two-service compose (web + redis,
// prebuilt images - no clone/build needed, so it runs in this sandbox),
// then confirms from INSIDE the web container that the redis peer resolves
// by its BARE service name "redis" - the exact check TEST-014 ran
// (nslookup) and found NXDOMAIN on before CreateContainerOpts/
// ConnectNetworks grew an aliases parameter and deployStack started
// passing svc.Name as that alias. Skips (not fails) if no Docker daemon is
// reachable, same gating as internal/tests/repository/docker_client_test.go.
func TestProjectServiceDeployStackServiceNameAlias(t *testing.T) {
	docker, err := dockerclient.New()
	if err != nil {
		t.Skipf("docker client not available: %v", err)
	}
	ctx := context.Background()
	if _, err := docker.DockerInfo(ctx); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}

	svc, _ := newTestProjectService(t)
	svc.docker = docker

	deployCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	project := &domain.Project{
		Name:       "alias-test-" + t.Name(),
		SourceType: domain.SourceTypeRemote,
	}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatalf("create project row: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		netName := projectNetworkName(project.ID)
		for _, service := range []string{"web", "redis"} {
			name := serviceContainerName(project.ID, service)
			docker.StopContainer(cleanupCtx, name)
			docker.RemoveContainer(cleanupCtx, name)
		}
		docker.NetworkRemove(cleanupCtx, netName)
	})

	// nginx:alpine (not bare alpine) for "web" - its default CMD keeps the
	// container running in the foreground so there's a live process to
	// exec into afterward; matches TEST-014's own web+redis fixture choice.
	// pullImages=true so deployStack pulls both images itself, same as a
	// real compose deploy would.
	services := []domain.ComposeService{
		{Name: "redis", Image: "redis:7-alpine"},
		{Name: "web", Image: "nginx:alpine", DependsOn: []string{"redis"}},
	}

	if err := svc.deployStack(deployCtx, project, services, true); err != nil {
		t.Fatalf("deployStack: %v", err)
	}

	webName := serviceContainerName(project.ID, "web")
	execID, err := docker.ExecCreate(deployCtx, webName, []string{"getent", "hosts", "redis"}, "")
	if err != nil {
		t.Fatalf("ExecCreate: %v", err)
	}
	hijacked, err := docker.ExecAttach(deployCtx, execID)
	if err != nil {
		t.Fatalf("ExecAttach: %v", err)
	}
	defer hijacked.Close()

	buf := make([]byte, 4096)
	n, _ := hijacked.Reader.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "redis") {
		t.Errorf("bare service-name alias %q did not resolve from peer web container; getent output: %q", "redis", output)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
