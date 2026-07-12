package service_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/traefik"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// newTestProjectService builds a ProjectService with a real throwaway
// SQLite DB, no Docker client (docker is nil, matching how the service
// behaves when Docker isn't available - deploy() bails out early via
// requireDocker) and a Traefik client pointed at a throwaway temp
// directory (RemoveRoute/AddRoute are only invoked when a project has a
// container, which the tests below avoid).
func newTestProjectService(t *testing.T) (*service.ProjectService, config.Config) {
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
	gitCred := service.NewGitCredentialService(db, "test-jwt-secret")

	return service.NewProjectService(db, nil, traefikClient, cfg, gitCred), cfg
}

// newTestProjectServiceWithDB is like newTestProjectService but returns the DB
// as the second value (for tests that need direct DB access).
func newTestProjectServiceWithDB(t *testing.T) (*service.ProjectService, *sqlite.DB) {
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
	gitCred := service.NewGitCredentialService(db, "test-jwt-secret")

	return service.NewProjectService(db, nil, traefikClient, cfg, gitCred), db
}

// TestProjectServiceCRUD covers create/read/update/delete, plus the env
// var CRUD that hangs off a project. Domain is left empty throughout so
// Delete()'s Traefik RemoveRoute call is just a no-op file removal.
func TestProjectServiceCRUD(t *testing.T) {
	svc, _ := newTestProjectService(t)
	ctx := context.Background()

	project, err := svc.Create(ctx, service.CreateProjectRequest{
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
	updated, err := svc.Update(ctx, project.ID, service.UpdateProjectRequest{Name: &newName})
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

func waitForProjectStatus(t *testing.T, svc *service.ProjectService, id int64, want domain.ProjectStatus) {
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

// TestProjectServiceUpdateExposedService verifies that updating exposed_service
// on a compose project rebinds the route to the new service (FEAT-040).
func TestProjectServiceUpdateExposedService(t *testing.T) {
	svc, _ := newTestProjectService(t)
	ctx := context.Background()

	// Create a compose project with two services
	composeYAML := `
version: '3'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
  api:
    image: python:3.9
    ports:
      - "8081:5000"
`
	project, err := svc.Create(ctx, service.CreateProjectRequest{
		Name:           "multi-service-app",
		SourceType:     domain.SourceTypeCompose,
		Domain:         "app.example.com",
		ComposeYAML:    composeYAML,
		ExposedService: "web", // Initially expose the web service
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if project.ExposedService != "web" {
		t.Fatalf("expected exposed_service 'web', got %q", project.ExposedService)
	}

	// Update the exposed_service to 'api'
	newExposedService := "api"
	updated, err := svc.Update(ctx, project.ID, service.UpdateProjectRequest{
		ExposedService: &newExposedService,
	})
	if err != nil {
		t.Fatalf("update exposed_service: %v", err)
	}
	if updated.ExposedService != "api" {
		t.Fatalf("expected exposed_service to be updated to 'api', got %q", updated.ExposedService)
	}

	// Verify the update was persisted
	retrieved, err := svc.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if retrieved.ExposedService != "api" {
		t.Fatalf("expected persisted exposed_service 'api', got %q", retrieved.ExposedService)
	}
}

// TestProjectServiceUpdateExposedServiceDomainAndService verifies that
// updating both domain and exposed_service in one call works correctly.
func TestProjectServiceUpdateExposedServiceDomainAndService(t *testing.T) {
	svc, _ := newTestProjectService(t)
	ctx := context.Background()

	composeYAML := `
version: '3'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
  api:
    image: python:3.9
    ports:
      - "8081:5000"
`
	project, err := svc.Create(ctx, service.CreateProjectRequest{
		Name:           "multi-service-app",
		SourceType:     domain.SourceTypeCompose,
		Domain:         "old.example.com",
		ComposeYAML:    composeYAML,
		ExposedService: "web",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Update both domain and exposed_service
	newDomain := "new.example.com"
	newExposedService := "api"
	updated, err := svc.Update(ctx, project.ID, service.UpdateProjectRequest{
		Domain:         &newDomain,
		ExposedService: &newExposedService,
	})
	if err != nil {
		t.Fatalf("update domain and exposed_service: %v", err)
	}
	if updated.Domain != "new.example.com" {
		t.Fatalf("expected domain 'new.example.com', got %q", updated.Domain)
	}
	if updated.ExposedService != "api" {
		t.Fatalf("expected exposed_service 'api', got %q", updated.ExposedService)
	}
}

// TestProjectServiceUpdateClearExposedService verifies that clearing
// exposed_service (setting to empty string) is handled correctly.
func TestProjectServiceUpdateClearExposedService(t *testing.T) {
	svc, _ := newTestProjectService(t)
	ctx := context.Background()

	composeYAML := `
version: '3'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
`
	project, err := svc.Create(ctx, service.CreateProjectRequest{
		Name:           "single-service-app",
		SourceType:     domain.SourceTypeCompose,
		Domain:         "app.example.com",
		ComposeYAML:    composeYAML,
		ExposedService: "web",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Clear exposed_service by setting it to empty string
	emptyService := ""
	updated, err := svc.Update(ctx, project.ID, service.UpdateProjectRequest{
		ExposedService: &emptyService,
	})
	if err != nil {
		t.Fatalf("clear exposed_service: %v", err)
	}
	if updated.ExposedService != "" {
		t.Fatalf("expected exposed_service to be cleared, got %q", updated.ExposedService)
	}
}

// TestProjectServiceUpdateExposedServiceNoRunningContainer verifies that
// updating exposed_service to a valid service that has no running container
// returns an error without persisting the change (keeps state consistent).
func TestProjectServiceUpdateExposedServiceNoRunningContainer(t *testing.T) {
	svc, db := newTestProjectServiceWithDB(t)
	ctx := context.Background()

	composeYAML := `
version: '3'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
  api:
    image: python:3.9
    ports:
      - "8081:5000"
`
	project, err := svc.Create(ctx, service.CreateProjectRequest{
		Name:           "multi-service-app",
		SourceType:     domain.SourceTypeCompose,
		Domain:         "app.example.com",
		ComposeYAML:    composeYAML,
		ExposedService: "web",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Manually set ContainerID to simulate deployment
	project.ContainerID = "test-container-id"
	if err := db.UpdateProject(project); err != nil {
		t.Fatalf("update project to set container ID: %v", err)
	}

	// Create a service container for "web" only (simulating that api has no container)
	webContainer := &domain.ServiceContainer{
		ProjectID:     project.ID,
		ServiceName:   "web",
		ContainerID:   "web-container-id",
		ContainerName: fmt.Sprintf("project-%d-web", project.ID),
		Status:        "running",
	}
	if err := db.ReplaceServiceContainers(project.ID, []*domain.ServiceContainer{webContainer}); err != nil {
		t.Fatalf("create web service container: %v", err)
	}

	// Try to update exposed_service to "api" (which has no running container)
	newExposedService := "api"
	_, err = svc.Update(ctx, project.ID, service.UpdateProjectRequest{
		ExposedService: &newExposedService,
	})
	if err == nil {
		t.Fatal("expected error when updating to service with no running container")
	}
	if !strings.Contains(err.Error(), "no running container") {
		t.Errorf("expected error to mention 'no running container', got: %v", err)
	}

	// Verify that the persisted exposed_service was NOT changed
	retrieved, err := svc.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("get after failed update: %v", err)
	}
	if retrieved.ExposedService != "web" {
		t.Errorf("expected exposed_service to remain 'web' after failed update, got %q", retrieved.ExposedService)
	}
}

// TestProjectServiceUpdateNameNoErrorWhenServiceDown verifies that updating
// other fields (like Name) doesn't fail even if the exposed_service is down.
// The error only applies when explicitly rebinding to a service with no container.
func TestProjectServiceUpdateNameNoErrorWhenServiceDown(t *testing.T) {
	svc, db := newTestProjectServiceWithDB(t)
	ctx := context.Background()

	composeYAML := `
version: '3'
services:
  web:
    image: nginx:latest
`
	project, err := svc.Create(ctx, service.CreateProjectRequest{
		Name:           "app",
		SourceType:     domain.SourceTypeCompose,
		Domain:         "app.example.com",
		ComposeYAML:    composeYAML,
		ExposedService: "web",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Set ContainerID to simulate deployment
	project.ContainerID = "test-container-id"
	if err := db.UpdateProject(project); err != nil {
		t.Fatalf("update project: %v", err)
	}

	// Note: we DON'T create any service containers, so resolveExposedUpstream will fail
	// But updating just the Name should still succeed (not explicitly rebinding)

	newName := "renamed-app"
	updated, err := svc.Update(ctx, project.ID, service.UpdateProjectRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("update name should succeed even if exposed_service is down: %v", err)
	}
	if updated.Name != newName {
		t.Errorf("expected name to be updated to %q, got %q", newName, updated.Name)
	}
	// Exposed service should remain unchanged since we didn't try to rebind
	if updated.ExposedService != "web" {
		t.Errorf("expected exposed_service to remain 'web', got %q", updated.ExposedService)
	}
}
