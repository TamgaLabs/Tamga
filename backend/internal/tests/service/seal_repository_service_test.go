package service_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestSealRepositoryLifecycleUsesAtomicOwnedCheckouts(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "seals.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	dataDir := t.TempDir()
	svc := service.NewSealService(db, config.Config{DataDir: dataDir})
	seal, err := svc.Create(context.Background(), service.CreateSealRequest{Name: "multi", Domain: "multi.test"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}

	remoteA := createGitFixture(t, "api", "api.txt", "first")
	remoteB := createGitFixture(t, "worker", "worker.txt", "second")
	api, err := svc.CreateRepository(context.Background(), seal.ID, service.CreateSealRepositoryRequest{DisplayName: "api", RemoteURL: remoteA})
	if err != nil {
		t.Fatalf("create api repository: %v", err)
	}
	worker, err := svc.CreateRepository(context.Background(), seal.ID, service.CreateSealRepositoryRequest{DisplayName: "worker", RemoteURL: remoteB, Branch: "main"})
	if err != nil {
		t.Fatalf("create worker repository: %v", err)
	}
	if api.Status != domain.ProjectSourceStatusReady || worker.Status != domain.ProjectSourceStatusReady {
		t.Fatalf("expected ready repositories, got api=%q worker=%q", api.Status, worker.Status)
	}
	if api.WorkspacePath != "repositories/api" || worker.WorkspacePath != "repositories/worker" {
		t.Fatalf("unexpected owned repository paths: api=%q worker=%q", api.WorkspacePath, worker.WorkspacePath)
	}
	apiCheckout := filepath.Join(dataDir, "seals", "1", "repositories", "api")
	if content, err := os.ReadFile(filepath.Join(apiCheckout, "api.txt")); err != nil || string(content) != "first" {
		t.Fatalf("read api checkout: content=%q err=%v", content, err)
	}

	missingRemote := remoteA + ".missing"
	if err := os.Rename(remoteA, missingRemote); err != nil {
		t.Fatalf("hide fixture remote: %v", err)
	}
	refreshed, err := svc.RefreshRepository(context.Background(), seal.ID, api.ID)
	if err != nil {
		t.Fatalf("refresh failure must remain observable rather than return an internal error: %v", err)
	}
	if refreshed.Status != domain.ProjectSourceStatusCloneFailed || refreshed.ErrorSummary != "unable to refresh repository" {
		t.Fatalf("unexpected failed refresh state: %+v", refreshed)
	}
	if content, err := os.ReadFile(filepath.Join(apiCheckout, "api.txt")); err != nil || string(content) != "first" {
		t.Fatalf("failed refresh destroyed prior checkout: content=%q err=%v", content, err)
	}

	repositories, err := svc.ListRepositories(context.Background(), seal.ID)
	if err != nil || len(repositories) != 2 {
		t.Fatalf("list repositories: count=%d err=%v", len(repositories), err)
	}
	if _, err := svc.CreateRepository(context.Background(), seal.ID, service.CreateSealRepositoryRequest{DisplayName: "../escape", RemoteURL: remoteB}); err == nil {
		t.Fatal("expected unsafe repository name to be rejected")
	}
	if err := svc.DeleteRepository(context.Background(), seal.ID, worker.ID); err != nil {
		t.Fatalf("delete worker repository: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "seals", "1", "repositories", "worker")); !os.IsNotExist(err) {
		t.Fatalf("deleted repository checkout remains: %v", err)
	}
}

func createGitFixture(t *testing.T, name, filename, content string) string {
	t.Helper()
	root := t.TempDir()
	worktree := filepath.Join(root, name)
	remote := filepath.Join(root, name+".git")
	runGit(t, root, "init", "--initial-branch=main", worktree)
	if err := os.WriteFile(filepath.Join(worktree, filename), []byte(content), 0644); err != nil {
		t.Fatalf("write git fixture: %v", err)
	}
	runGit(t, worktree, "add", filename)
	runGit(t, worktree, "-c", "user.name=Tamga test", "-c", "user.email=test@tamga.invalid", "commit", "-m", "fixture")
	runGit(t, root, "init", "--bare", remote)
	runGit(t, worktree, "remote", "add", "origin", remote)
	runGit(t, worktree, "push", "origin", "main")
	return remote
}

func runGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
