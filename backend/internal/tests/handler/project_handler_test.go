package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/handler"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/traefik"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func newTestProjectService(t *testing.T) (*service.ProjectService, *sqlite.DB) {
	t.Helper()
	dbPath := "/tmp/test_project_handler_" + t.Name() + ".db"
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
	gitCred := service.NewGitCredentialService(db, "test-jwt-secret")

	return service.NewProjectService(db, nil, traefikClient, cfg, gitCred), db
}

func setupRouter(h *handler.ProjectHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/projects", h.Create)
	r.Get("/projects/{id}", h.Get)
	r.Put("/projects/{id}", h.Update)
	r.Delete("/projects/{id}", h.Delete)
	r.Post("/projects/{id}/restart", h.Restart)
	r.Get("/projects/{id}/logs", h.Logs)
	return r
}

func TestProjectHandler_NotFound(t *testing.T) {
	svc, _ := newTestProjectService(t)
	h := handler.NewProjectHandler(svc)
	r := setupRouter(h)

	nonexistentID := "999999"

	tests := []struct {
		name           string
		method         string
		url            string
		body           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Get nonexistent project",
			method:         "GET",
			url:            "/projects/" + nonexistentID,
			body:           "",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "not found\n",
		},
		{
			name:           "Update nonexistent project",
			method:         "PUT",
			url:            "/projects/" + nonexistentID,
			body:           `{"name":"new-name"}`,
			expectedStatus: http.StatusNotFound,
			expectedBody:   "not found\n",
		},
		{
			name:           "Delete nonexistent project",
			method:         "DELETE",
			url:            "/projects/" + nonexistentID,
			body:           "",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "not found\n",
		},
		{
			name:           "Restart nonexistent project",
			method:         "POST",
			url:            "/projects/" + nonexistentID + "/restart",
			body:           "",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "not found\n",
		},
		{
			name:           "Logs nonexistent project",
			method:         "GET",
			url:            "/projects/" + nonexistentID + "/logs",
			body:           "",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "not found\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.url, bytes.NewBufferString(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.url, nil)
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if w.Body.String() != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestProjectHandler_RealProject(t *testing.T) {
	svc, db := newTestProjectService(t)
	h := handler.NewProjectHandler(svc)
	r := setupRouter(h)

	// Pre-populate a project in the database.
	proj := &domain.Project{
		Name:       "existing-project",
		SourceType: domain.SourceTypeLocal,
		Status:     domain.ProjectStatusCreated,
	}
	if err := db.CreateProject(proj); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	idStr := strconv.FormatInt(proj.ID, 10)

	// Test GET existing project
	t.Run("Get existing project", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/projects/"+idStr, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var p domain.Project
		if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if p.Name != proj.Name {
			t.Errorf("expected project name %q, got %q", proj.Name, p.Name)
		}
	})

	// Test PUT existing project
	t.Run("Update existing project", func(t *testing.T) {
		reqBody := `{"name":"updated-project"}`
		req := httptest.NewRequest("PUT", "/projects/"+idStr, bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var p domain.Project
		if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if p.Name != "updated-project" {
			t.Errorf("expected project name to be 'updated-project', got %q", p.Name)
		}
	})

	// Test DELETE existing project
	t.Run("Delete existing project", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/projects/"+idStr, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", w.Code)
		}

		// Verify it was actually deleted from database.
		_, err := db.FindProject(proj.ID)
		if err == nil {
			t.Error("expected project to be deleted, but it still exists")
		}
	})
}

// TestProjectHandler_Create_ComposeValidation covers FEAT-029's
// create-surface validation (carried from FEAT-028's review finding): a
// bad compose_yaml or a stale exposed_service must be rejected inline,
// synchronously, with a 400 - not silently discovered later by the async
// deploy() goroutine. Docker is nil in this test service (see
// newTestProjectService), so a successful create's deploy() goroutine
// fails fast via requireDocker and lands in ProjectStatusError - that's
// expected and doesn't affect what Create() itself returns synchronously.
func TestProjectHandler_Create_ComposeValidation(t *testing.T) {
	svc, _ := newTestProjectService(t)
	h := handler.NewProjectHandler(svc)
	r := setupRouter(h)

	post := func(t *testing.T, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", "/projects", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("unsupported compose feature rejected inline", func(t *testing.T) {
		body := `{"name":"bad-compose","domain":"bad.example.com","compose_yaml":"services:\n  web:\n    build: .\n"}`
		w := post(t, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "build") {
			t.Errorf("expected error message to mention the unsupported build: feature, got %q", w.Body.String())
		}
	})

	t.Run("unknown exposed_service rejected inline", func(t *testing.T) {
		body := `{"name":"stale-exposed","domain":"stale.example.com","compose_yaml":"services:\n  web:\n    image: nginx:latest\n","exposed_service":"does-not-exist"}`
		w := post(t, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "does-not-exist") {
			t.Errorf("expected error message to name the stale exposed_service, got %q", w.Body.String())
		}
	})

	t.Run("valid compose create succeeds and persists compose fields", func(t *testing.T) {
		body := `{"name":"good-compose","domain":"good.example.com","compose_yaml":"services:\n  web:\n    image: nginx:latest\n  db:\n    image: postgres:16\n","exposed_service":"web"}`
		w := post(t, body)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d (body: %s)", w.Code, w.Body.String())
		}
		var p domain.Project
		if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if p.SourceType != domain.SourceTypeCompose {
			t.Errorf("expected source_type %q, got %q", domain.SourceTypeCompose, p.SourceType)
		}
		if p.ComposeYAML == "" {
			t.Error("expected compose_yaml to be persisted on the project")
		}
		if p.ExposedService != "web" {
			t.Errorf("expected exposed_service %q, got %q", "web", p.ExposedService)
		}
	})

	t.Run("git-repo create path unchanged: repo_url still required for remote source", func(t *testing.T) {
		body := `{"name":"git-project","domain":"git.example.com","source_type":"remote"}`
		w := post(t, body)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "repo_url") {
			t.Errorf("expected error message to mention repo_url, got %q", w.Body.String())
		}
	})

	t.Run("git-repo create path unchanged: valid remote create succeeds", func(t *testing.T) {
		body := `{"name":"git-project-2","domain":"git2.example.com","source_type":"remote","repo_url":"https://example.invalid/org/repo.git"}`
		w := post(t, body)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d (body: %s)", w.Code, w.Body.String())
		}
		var p domain.Project
		if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if p.ComposeYAML != "" {
			t.Errorf("expected empty compose_yaml on a git-repo create, got %q", p.ComposeYAML)
		}
	})
}

// TestProjectHandler_Update_ExposedServiceValidation covers FEAT-040's
// exposed_service validation on Update: rebinding to an invalid service
// must be rejected inline with a 400, validating against the project's
// compose_yaml services (if it's a compose project).
func TestProjectHandler_Update_ExposedServiceValidation(t *testing.T) {
	svc, db := newTestProjectService(t)
	h := handler.NewProjectHandler(svc)
	r := setupRouter(h)

	// Create a compose project with multiple services
	composeYAML := `services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
  api:
    image: python:3.9
    ports:
      - "8081:5000"
`
	proj := &domain.Project{
		Name:           "multi-service-app",
		SourceType:     domain.SourceTypeCompose,
		Domain:         "app.example.com",
		ComposeYAML:    composeYAML,
		ExposedService: "web",
		Status:         domain.ProjectStatusCreated,
	}
	if err := db.CreateProject(proj); err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	idStr := strconv.FormatInt(proj.ID, 10)

	t.Run("rebind to valid service in compose succeeds", func(t *testing.T) {
		reqBody := `{"exposed_service":"api"}`
		req := httptest.NewRequest("PUT", "/projects/"+idStr, bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
		}

		var p domain.Project
		if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if p.ExposedService != "api" {
			t.Errorf("expected exposed_service to be updated to 'api', got %q", p.ExposedService)
		}
	})

	t.Run("rebind to invalid service in compose rejected with 400", func(t *testing.T) {
		reqBody := `{"exposed_service":"does-not-exist"}`
		req := httptest.NewRequest("PUT", "/projects/"+idStr, bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d (body: %s)", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "does-not-exist") {
			t.Errorf("expected error message to name the invalid exposed_service, got %q", w.Body.String())
		}
	})

	t.Run("update other fields unaffected by exposed_service validation", func(t *testing.T) {
		newName := "renamed-app"
		reqBody := `{"name":"renamed-app"}`
		req := httptest.NewRequest("PUT", "/projects/"+idStr, bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
		}

		var p domain.Project
		if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if p.Name != newName {
			t.Errorf("expected name to be updated to %q, got %q", newName, p.Name)
		}
	})

	t.Run("git-based project allows any exposed_service (no validation for non-compose)", func(t *testing.T) {
		// Create a git-based project (no compose_yaml)
		gitProj := &domain.Project{
			Name:       "git-based-app",
			SourceType: domain.SourceTypeRemote,
			RepoURL:    "https://example.invalid/org/repo.git",
			Domain:     "git.example.com",
			Status:     domain.ProjectStatusCreated,
		}
		if err := db.CreateProject(gitProj); err != nil {
			t.Fatalf("failed to create git project: %v", err)
		}

		gitIDStr := strconv.FormatInt(gitProj.ID, 10)

		// Update with an arbitrary exposed_service should succeed (no compose to validate against)
		reqBody := `{"exposed_service":"any-service"}`
		req := httptest.NewRequest("PUT", "/projects/"+gitIDStr, bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 for git-based project, got %d (body: %s)", w.Code, w.Body.String())
		}
	})
}
