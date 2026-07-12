package service_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
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

// newTestProjectServiceWithRealDocker builds a ProjectService wired to a
// real Docker daemon (skipping the test if none is reachable - same
// gating pattern as docker_client_test.go's newTestDockerClient) instead
// of the nil docker client every other test in this file uses. BUG-033's
// running-state check (exposedServiceRunning) only has anything to
// inspect when a real docker client is present - with a nil client it
// falls back to "does a row exist", which the nil-docker tests above
// already cover.
func newTestProjectServiceWithRealDocker(t *testing.T) (*service.ProjectService, *sqlite.DB, *dockerclient.Client) {
	t.Helper()
	docker, err := dockerclient.New()
	if err != nil {
		t.Skipf("docker client not available: %v", err)
	}
	if _, err := docker.DockerInfo(context.Background()); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}

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

	return service.NewProjectService(db, docker, traefikClient, cfg, gitCred), db, docker
}

// TestProjectServiceUpdateExposedServiceStoppedContainer is BUG-033: a
// project_service_containers ROW existing for a service is not the same
// as that service's container actually being RUNNING (resolveExposedUpstream
// alone can't tell the two apart, by design - see exposedServiceRunning's
// doc comment). Rebinding to a service whose container exists but is not
// running must be rejected (the "no running container" error the handler
// maps to 409) rather than silently moving the route to a container that
// will 502 - and rebinding to an actually running service must still
// work. Uses a real Docker daemon (skips if unavailable) since this is
// exactly the distinction that requires an actual container inspection.
func TestProjectServiceUpdateExposedServiceStoppedContainer(t *testing.T) {
	svc, db, docker := newTestProjectServiceWithRealDocker(t)
	ctx := context.Background()

	const image = "redis:7-alpine"
	if err := docker.PullImage(ctx, image); err != nil {
		t.Fatalf("PullImage(%s): %v", image, err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	netName := "tamga-test-rebind-net-" + suffix
	if err := docker.EnsureNetwork(ctx, netName, true); err != nil {
		t.Fatalf("EnsureNetwork: %v", err)
	}
	t.Cleanup(func() { docker.NetworkRemove(context.Background(), netName) })

	// "web" - created AND started, stays running for the whole test.
	webName := "tamga-test-rebind-web-" + suffix
	webID, err := docker.CreateContainerOpts(ctx, webName, image, nil, netName, nil, container.Resources{}, false, nil)
	if err != nil {
		t.Fatalf("CreateContainerOpts(web): %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		docker.StopContainer(cleanupCtx, webID)
		docker.RemoveContainer(cleanupCtx, webID)
	})
	if err := docker.StartContainer(ctx, webID); err != nil {
		t.Fatalf("StartContainer(web): %v", err)
	}

	// "web2" - created but never started, matching a `docker stop`ped (or
	// never-started) service that still has its persisted
	// project_service_containers row.
	web2Name := "tamga-test-rebind-web2-" + suffix
	web2ID, err := docker.CreateContainerOpts(ctx, web2Name, image, nil, netName, nil, container.Resources{}, false, nil)
	if err != nil {
		t.Fatalf("CreateContainerOpts(web2): %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		docker.StopContainer(cleanupCtx, web2ID)
		docker.RemoveContainer(cleanupCtx, web2ID)
	})

	project := &domain.Project{
		Name:           "rebind-stopped-test",
		SourceType:     domain.SourceTypeCompose,
		Domain:         "rebind-stopped.example.com",
		ExposedService: "web",
		Status:         domain.ProjectStatusRunning,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	// CreateProject always persists container_id='' regardless of the
	// struct field (see its doc comment) - ContainerID has to be set via
	// a follow-up UpdateProject, same as
	// TestProjectServiceUpdateExposedServiceNoRunningContainer does.
	project.ContainerID = webID
	if err := db.UpdateProject(project); err != nil {
		t.Fatalf("update project to set container ID: %v", err)
	}

	containers := []*domain.ServiceContainer{
		{ProjectID: project.ID, ServiceName: "web", ContainerID: webID, ContainerName: webName, Status: "running"},
		{ProjectID: project.ID, ServiceName: "web2", ContainerID: web2ID, ContainerName: web2Name, Status: "created"},
	}
	if err := db.ReplaceServiceContainers(project.ID, containers); err != nil {
		t.Fatalf("replace service containers: %v", err)
	}

	// Rebinding to "web2" (not running) must be rejected, not accepted
	// with the route silently moved to a down container.
	target := "web2"
	_, err = svc.Update(ctx, project.ID, service.UpdateProjectRequest{ExposedService: &target})
	if err == nil {
		t.Fatal("expected error rebinding to a service whose container is not running")
	}
	if !strings.Contains(err.Error(), "no running container") {
		t.Errorf("expected error to mention 'no running container', got: %v", err)
	}
	retrieved, err := svc.Get(ctx, project.ID)
	if err != nil {
		t.Fatalf("get after rejected rebind: %v", err)
	}
	if retrieved.ExposedService != "web" {
		t.Errorf("expected exposed_service to remain 'web' after rejected rebind, got %q", retrieved.ExposedService)
	}

	// Now start web2 and rebind again - the common case (rebind to an
	// actually running service) must still succeed (no TEST-018
	// regression).
	if err := docker.StartContainer(ctx, web2ID); err != nil {
		t.Fatalf("StartContainer(web2): %v", err)
	}
	updated, err := svc.Update(ctx, project.ID, service.UpdateProjectRequest{ExposedService: &target})
	if err != nil {
		t.Fatalf("expected rebind to a running service to succeed, got: %v", err)
	}
	if updated.ExposedService != "web2" {
		t.Errorf("expected exposed_service 'web2' after successful rebind, got %q", updated.ExposedService)
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
