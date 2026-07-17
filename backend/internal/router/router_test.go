package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/handler"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestSealRoutesCreateEmptySealAndExcludeProjectRoutes(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "tamga.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	sealHandler := handler.NewSealHandler(service.NewSealService(db, config.Config{DataDir: t.TempDir()}))
	r := New(nil, nil, sealHandler, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(next http.Handler) http.Handler {
		return next
	})

	request := httptest.NewRequest(http.MethodPost, "/api/seals", bytes.NewBufferString(`{"name":"empty seal","domain":"empty.example.test"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("POST /api/seals status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	var seal domain.Seal
	if err := json.NewDecoder(response.Body).Decode(&seal); err != nil {
		t.Fatalf("decode seal response: %v", err)
	}
	if seal.ID == 0 || seal.Name != "empty seal" || seal.Domain != "empty.example.test" {
		t.Fatalf("unexpected seal response: %+v", seal)
	}
	if seal.SourceType != domain.SourceTypeEmpty || seal.Status != domain.ProjectStatusConfiguring {
		t.Fatalf("empty Seal lifecycle = source_type %q status %q, want %q and %q", seal.SourceType, seal.Status, domain.SourceTypeEmpty, domain.ProjectStatusConfiguring)
	}

	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/seals", bytes.NewBufferString(`{"domain":"empty.example.test"}`))
	invalidResponse := httptest.NewRecorder()
	r.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/seals without name status = %d, want %d", invalidResponse.Code, http.StatusBadRequest)
	}

	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	legacyResponse := httptest.NewRecorder()
	r.ServeHTTP(legacyResponse, legacyRequest)
	if legacyResponse.Code != http.StatusNotFound {
		t.Fatalf("GET /api/projects status = %d, want %d", legacyResponse.Code, http.StatusNotFound)
	}
}

func TestSealRepositoryRoutesLifecycle(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "tamga.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	dataDir := t.TempDir()
	svc := service.NewSealService(db, config.Config{DataDir: dataDir})
	seal, err := svc.Create(t.Context(), service.CreateSealRequest{Name: "repos"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}
	if seal.ID != 1 {
		t.Fatalf("unexpected seal id: %d", seal.ID)
	}
	sealHandler := handler.NewSealHandler(svc)
	r := New(nil, nil, sealHandler, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(next http.Handler) http.Handler {
		return next
	})

	remote := routerGitFixture(t)
	create := httptest.NewRequest(http.MethodPost, "/api/seals/1/repositories", bytes.NewBufferString(`{"display_name":"api","remote_url":"`+remote+`"}`))
	create.Header.Set("Content-Type", "application/json")
	created := httptest.NewRecorder()
	r.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("POST repository status = %d, want %d: %s", created.Code, http.StatusCreated, created.Body.String())
	}
	var repository domain.SealRepository
	if err := json.NewDecoder(created.Body).Decode(&repository); err != nil {
		t.Fatalf("decode repository: %v", err)
	}
	if repository.Status != domain.ProjectSourceStatusReady || repository.WorkspacePath != "repositories/api" {
		t.Fatalf("unexpected created repository: %+v", repository)
	}

	listed := httptest.NewRecorder()
	r.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/api/seals/1/repositories", nil))
	if listed.Code != http.StatusOK {
		t.Fatalf("GET repositories status = %d, want %d: %s", listed.Code, http.StatusOK, listed.Body.String())
	}
	var repositories []domain.SealRepository
	if err := json.NewDecoder(listed.Body).Decode(&repositories); err != nil || len(repositories) != 1 {
		t.Fatalf("decode repository list: count=%d err=%v", len(repositories), err)
	}

	deleted := httptest.NewRecorder()
	r.ServeHTTP(deleted, httptest.NewRequest(http.MethodDelete, "/api/seals/1/repositories/"+strconv.FormatInt(repository.ID, 10), nil))
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("DELETE repository status = %d, want %d: %s", deleted.Code, http.StatusNoContent, deleted.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dataDir, "seals", "1", "repositories", "api")); !os.IsNotExist(err) {
		t.Fatalf("deleted repository checkout remains: %v", err)
	}
}

func routerGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	worktree := filepath.Join(root, "worktree")
	remote := filepath.Join(root, "remote.git")
	routerGit(t, root, "init", "--initial-branch=main", worktree)
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("fixture"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	routerGit(t, worktree, "add", "README.md")
	routerGit(t, worktree, "-c", "user.name=Tamga test", "-c", "user.email=test@tamga.invalid", "commit", "-m", "fixture")
	routerGit(t, root, "init", "--bare", remote)
	routerGit(t, worktree, "remote", "add", "origin", remote)
	routerGit(t, worktree, "push", "origin", "main")
	return remote
}

func routerGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
