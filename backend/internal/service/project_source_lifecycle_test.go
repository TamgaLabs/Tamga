package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// TestCloneSourcesReplacesOwnedDirectoryAndRedactsFailure drives the source
// lifecycle synchronously with a controlled clone-command seam. It proves the
// production directory-replacement boundary without a network clone or
// goroutine scheduling dependency.
func TestCloneSourcesReplacesOwnedDirectoryAndRedactsFailure(t *testing.T) {
	svc, cfg := newTestProjectService(t)
	project := &domain.Project{Name: "multi", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	source := &domain.ProjectSource{ProjectID: project.ID, DisplayName: "worker", RemoteURL: "https://example.test/worker.git", Branch: "main", WorkspacePath: "sources/worker", Status: domain.ProjectSourceStatusPending}
	if err := svc.db.CreateProjectSource(source); err != nil {
		t.Fatalf("create source: %v", err)
	}
	workDir := filepath.Join(cfg.DataDir, "projects", fmt.Sprintf("%d", project.ID), "sources", "worker")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatalf("seed old checkout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "stale.txt"), []byte("stale"), 0644); err != nil {
		t.Fatalf("write stale checkout marker: %v", err)
	}

	svc.runCloneCommand = func(_ context.Context, cloneURL, branch, dir string) error {
		if cloneURL != source.RemoteURL || branch != source.Branch || dir != workDir {
			t.Fatalf("unexpected clone command: url=%q branch=%q dir=%q", cloneURL, branch, dir)
		}
		if _, err := os.Stat(filepath.Join(dir, "stale.txt")); !os.IsNotExist(err) {
			t.Fatalf("expected owned directory replacement before clone, stale marker err=%v", err)
		}
		return os.WriteFile(filepath.Join(dir, "fresh.txt"), []byte("fresh"), 0644)
	}
	svc.cloneSources(context.Background(), project.ID, source.ID)

	got, err := svc.db.FindProjectSource(project.ID, source.ID)
	if err != nil {
		t.Fatalf("find ready source: %v", err)
	}
	if got.Status != domain.ProjectSourceStatusReady {
		t.Fatalf("expected ready source, got %q", got.Status)
	}
	if _, err := os.Stat(filepath.Join(workDir, "fresh.txt")); err != nil {
		t.Fatalf("expected replacement checkout content: %v", err)
	}

	svc.runCloneCommand = func(context.Context, string, string, string) error {
		return fmt.Errorf("authentication failed for https://token:secret@example.test/private.git")
	}
	svc.cloneSources(context.Background(), project.ID, source.ID)

	got, err = svc.db.FindProjectSource(project.ID, source.ID)
	if err != nil {
		t.Fatalf("find failed source: %v", err)
	}
	if got.Status != domain.ProjectSourceStatusCloneFailed || got.ErrorSummary != "unable to clone source" {
		t.Fatalf("expected credential-safe clone failure, got status=%q error=%q", got.Status, got.ErrorSummary)
	}
	storedProject, err := svc.db.FindProject(project.ID)
	if err != nil {
		t.Fatalf("find failed project: %v", err)
	}
	if storedProject.Status != domain.ProjectStatusCloneFailed {
		t.Fatalf("expected project clone_failed, got %q", storedProject.Status)
	}
}
