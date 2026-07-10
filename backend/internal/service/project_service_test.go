package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/caddy"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
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
