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
	db, r, _ := newSealRouter(t)
	defer db.Close()

	request := httptest.NewRequest(http.MethodPost, "/api/seals", bytes.NewBufferString(`{"name":"empty seal","domain":"empty.example.test"}`))
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("POST /api/seals status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	var seal domain.Seal
	if err := json.NewDecoder(response.Body).Decode(&seal); err != nil {
		t.Fatalf("decode seal response: %v", err)
	}
	if seal.ID == 0 || seal.Name != "empty seal" {
		t.Fatalf("unexpected empty seal: %+v", seal)
	}

	legacy := httptest.NewRecorder()
	r.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/projects", nil))
	if legacy.Code != http.StatusNotFound {
		t.Fatalf("GET /api/projects status = %d, want 404", legacy.Code)
	}
}

func TestSealProjectRoutesImportRefreshAndServiceConfiguration(t *testing.T) {
	db, r, svc := newSealRouter(t)
	defer db.Close()
	seal, err := svc.Create(t.Context(), service.CreateSealRequest{Name: "projects"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}

	remote := routerGitFixture(t)
	projectsPath := "/api/seals/" + strconv.FormatInt(seal.ID, 10) + "/projects"
	created := serveJSON(r, http.MethodPost, projectsPath, `{"name":"app","remote_url":"`+remote+`"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("POST project status = %d, body=%s", created.Code, created.Body.String())
	}
	var project domain.Project
	if err := json.NewDecoder(created.Body).Decode(&project); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	if project.Status != domain.ProjectStatusCreated || project.SealID != seal.ID {
		t.Fatalf("unexpected imported project: %+v", project)
	}

	listed := serveJSON(r, http.MethodGet, projectsPath, "")
	var projects []domain.Project
	if listed.Code != http.StatusOK || json.NewDecoder(listed.Body).Decode(&projects) != nil || len(projects) != 1 {
		t.Fatalf("GET projects status=%d body=%s", listed.Code, listed.Body.String())
	}

	projectPath := projectsPath + "/" + strconv.FormatInt(project.ID, 10)
	refreshed := serveJSON(r, http.MethodPost, projectPath+"/refresh", "")
	if refreshed.Code != http.StatusOK {
		t.Fatalf("POST refresh status=%d body=%s", refreshed.Code, refreshed.Body.String())
	}
	if err := json.NewDecoder(refreshed.Body).Decode(&project); err != nil || project.Status != domain.ProjectStatusConfiguring {
		t.Fatalf("refreshed project=%+v err=%v", project, err)
	}

	servicesPath := projectPath + "/services"
	serviceResponse := serveJSON(r, http.MethodPost, servicesPath, `{"name":"web","build_context":".","internal_port":3000}`)
	if serviceResponse.Code != http.StatusCreated {
		t.Fatalf("POST service status=%d body=%s", serviceResponse.Code, serviceResponse.Body.String())
	}
	var projectService domain.Service
	if err := json.NewDecoder(serviceResponse.Body).Decode(&projectService); err != nil {
		t.Fatalf("decode service: %v", err)
	}

	routesPath := servicesPath + "/" + strconv.FormatInt(projectService.ID, 10) + "/routes"
	routeResponse := serveJSON(r, http.MethodPost, routesPath, `{"domain":" App.Example.Test "}`)
	if routeResponse.Code != http.StatusCreated {
		t.Fatalf("POST route status=%d body=%s", routeResponse.Code, routeResponse.Body.String())
	}
	var route domain.ServiceRoute
	if err := json.NewDecoder(routeResponse.Body).Decode(&route); err != nil || route.Domain != "app.example.test" {
		t.Fatalf("route=%+v err=%v", route, err)
	}

	configuration := serveJSON(r, http.MethodGet, projectPath+"/configuration", "")
	if configuration.Code != http.StatusOK {
		t.Fatalf("GET configuration status=%d body=%s", configuration.Code, configuration.Body.String())
	}
	deleted := serveJSON(r, http.MethodDelete, projectPath, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("DELETE project status=%d body=%s", deleted.Code, deleted.Body.String())
	}
}

func newSealRouter(t *testing.T) (*sqlite.DB, http.Handler, *service.SealService) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "tamga.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Migrate(); err != nil {
		db.Close()
		t.Fatalf("migrate db: %v", err)
	}
	svc := service.NewSealService(db, config.Config{DataDir: t.TempDir()})
	sealHandler := handler.NewSealHandler(svc)
	return db, New(nil, nil, sealHandler, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(next http.Handler) http.Handler { return next }), svc
}

func serveJSON(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
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
