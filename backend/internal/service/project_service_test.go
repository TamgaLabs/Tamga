package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/caddy"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// newTestProjectService builds a ProjectService with a real throwaway
// SQLite DB, no Docker client (docker is nil, matching how the service
// behaves when Docker isn't available - deploy() bails out early via
// requireDocker) and a Caddy client pointed at an address nothing ever
// connects to (RemoveRoute is only invoked when a project has a non-empty
// Domain, which the tests below avoid).
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
	caddyClient := caddy.New("http://127.0.0.1:1")
	gitCred := NewGitCredentialService(db, "test-jwt-secret")

	return NewProjectService(db, nil, caddyClient, cfg, gitCred), cfg
}

// TestProjectServiceCRUD covers create/read/update/delete, plus the env
// var CRUD that hangs off a project. Domain is left empty throughout so
// Delete() never calls out to the (unreachable) Caddy admin API.
func TestProjectServiceCRUD(t *testing.T) {
	svc, _ := newTestProjectService(t)
	ctx := context.Background()

	project, err := svc.Create(ctx, CreateProjectRequest{
		Name:       "my-app",
		SourceType: domain.SourceTypeRemote,
		RepoURL:    "https://example.invalid/org/repo.git",
		Branch:     "main",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if project.ID == 0 {
		t.Fatal("expected project ID to be assigned")
	}
	if project.Name != "my-app" {
		t.Fatalf("expected name 'my-app', got %q", project.Name)
	}

	// Create() kicks off deployment in a background goroutine. Docker is
	// nil here, so it fails fast at requireDocker() and the project lands
	// in ProjectStatusError. Poll rather than sleep a fixed amount, since
	// this only needs to outlast goroutine scheduling, not any real I/O.
	waitForProjectStatus(t, svc, project.ID, domain.ProjectStatusError)

	// Read
	got, err := svc.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.RepoURL != "https://example.invalid/org/repo.git" {
		t.Fatalf("unexpected repo url: %q", got.RepoURL)
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 project, got %d", len(list))
	}

	// Update
	newName := "renamed-app"
	updated, err := svc.Update(ctx, project.ID, UpdateProjectRequest{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "renamed-app" {
		t.Fatalf("expected updated name 'renamed-app', got %q", updated.Name)
	}

	// Env vars hang off the project.
	ev, err := svc.CreateEnvVar(ctx, project.ID, "API_KEY", "secret-value")
	if err != nil {
		t.Fatalf("create env var: %v", err)
	}
	if ev.Key != "API_KEY" || ev.Value != "secret-value" {
		t.Fatalf("unexpected env var: %+v", ev)
	}

	envVars, err := svc.ListEnvVars(ctx, project.ID)
	if err != nil {
		t.Fatalf("list env vars: %v", err)
	}
	if len(envVars) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(envVars))
	}

	if err := svc.DeleteEnvVar(ctx, ev.ID); err != nil {
		t.Fatalf("delete env var: %v", err)
	}
	envVars, err = svc.ListEnvVars(ctx, project.ID)
	if err != nil {
		t.Fatalf("list env vars after delete: %v", err)
	}
	if len(envVars) != 0 {
		t.Fatalf("expected 0 env vars after delete, got %d", len(envVars))
	}

	// Recreate an env var so Delete's cascading cleanup below is exercised.
	if _, err := svc.CreateEnvVar(ctx, project.ID, "OTHER", "v"); err != nil {
		t.Fatalf("create second env var: %v", err)
	}

	// Delete
	if err := svc.Delete(ctx, project.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.Get(ctx, project.ID); err == nil {
		t.Fatal("expected error getting deleted project")
	}
	envVars, err = svc.ListEnvVars(ctx, project.ID)
	if err != nil {
		t.Fatalf("list env vars after project delete: %v", err)
	}
	if len(envVars) != 0 {
		t.Fatalf("expected env vars cascade-deleted with project, got %d", len(envVars))
	}
}

func waitForProjectStatus(t *testing.T, svc *ProjectService, id int64, want domain.ProjectStatus) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p, err := svc.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("get while polling status: %v", err)
		}
		if p.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for project %d to reach status %q", id, want)
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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
