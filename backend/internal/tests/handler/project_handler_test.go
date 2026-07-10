package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
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
