package sqlite_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// openTestDB opens a fresh throwaway SQLite DB at a temp path and migrates
// it, mirroring the pattern used throughout internal/tests/service.
func openTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestMigrationAppliesOnFreshDB covers FEAT-025's migration 000016: a brand
// new DB must migrate cleanly and end up with the two new project columns
// plus the project_service_containers table usable.
func TestMigrationAppliesOnFreshDB(t *testing.T) {
	db := openTestDB(t)

	// The two new project columns are reachable via CreateProject/FindProject
	// (exercised fully in TestProjectComposeColumnsRoundTrip below) - here
	// just confirm the child table exists and is queryable.
	if _, err := db.ListServiceContainers(1); err != nil {
		t.Fatalf("query project_service_containers on fresh db: %v", err)
	}
}

// TestMigrationAppliesOnCopiedLiveDB re-runs Migrate() against a throwaway
// copy of the actual on-disk dev database (never the live file itself,
// which openTestDB/this test never touches). Confirms migration 000016
// applies cleanly on a DB that already has real project rows created before
// this migration existed, and that those legacy rows come back with
// compose_yaml/exposed_service as empty strings (never a NULL-scan error).
// Skips if the live DB isn't present in this environment (e.g. CI) -
// mirrors the docker-daemon skip pattern used elsewhere in this suite.
func TestMigrationAppliesOnCopiedLiveDB(t *testing.T) {
	liveDBPath := filepath.Join("..", "..", "..", "..", "data", "tamga.db")
	if _, err := os.Stat(liveDBPath); err != nil {
		t.Skipf("live dev db not present at %s, skipping: %v", liveDBPath, err)
	}

	copyPath := filepath.Join(t.TempDir(), "tamga_copy.db")
	if err := copyFile(liveDBPath, copyPath); err != nil {
		t.Fatalf("copy live db: %v", err)
	}

	db, err := sqlite.Open(copyPath)
	if err != nil {
		t.Fatalf("open copied db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Raw COUNT rather than ListProjects(): the production ListProjects
	// query already references compose_yaml/exposed_service (it's the same
	// query regardless of which migrations have actually run), so it can
	// only be used post-migration here.
	var preExisting int
	if err := db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&preExisting); err != nil {
		t.Fatalf("count projects before migrate: %v", err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate copied live db: %v", err)
	}

	// Legacy rows (pre-dating migration 000016) must still read back fine -
	// NULL compose_yaml/exposed_service coalesce to "", not a scan error.
	postMigrate, err := db.ListProjects()
	if err != nil {
		t.Fatalf("list projects after migrate: %v", err)
	}
	if len(postMigrate) != preExisting {
		t.Fatalf("expected %d projects to survive migration unaffected, got %d", preExisting, len(postMigrate))
	}
	for _, p := range postMigrate {
		if p.ComposeYAML != "" {
			t.Errorf("legacy project %d: expected empty ComposeYAML, got %q", p.ID, p.ComposeYAML)
		}
		if p.ExposedService != "" {
			t.Errorf("legacy project %d: expected empty ExposedService, got %q", p.ID, p.ExposedService)
		}
	}

	// The child table must exist and be empty for legacy projects (nothing
	// migrated their containers into it - that's the deploy engine's job,
	// out of scope here).
	if len(postMigrate) > 0 {
		containers, err := db.ListServiceContainers(postMigrate[0].ID)
		if err != nil {
			t.Fatalf("list service containers for legacy project: %v", err)
		}
		if len(containers) != 0 {
			t.Fatalf("expected no service containers for a legacy project, got %d", len(containers))
		}
	}

	// Re-running Migrate() again must be a no-op (idempotent), same as every
	// other migration in this codebase.
	if err := db.Migrate(); err != nil {
		t.Fatalf("re-run migrate on already-migrated copied db: %v", err)
	}
}

// TestProjectComposeColumnsRoundTrip covers the new projects.compose_yaml /
// projects.exposed_service columns end to end: a freshly created project
// scans back as empty strings (not NULL), and UpdateProject persists
// non-empty values.
func TestProjectComposeColumnsRoundTrip(t *testing.T) {
	db := openTestDB(t)

	p := &domain.Project{
		Name:       "compose-app",
		SourceType: domain.SourceTypeRemote,
		RepoURL:    "https://example.invalid/org/repo.git",
		Branch:     "main",
		Domain:     "compose-app.example.com",
		Status:     domain.ProjectStatusCreated,
	}
	if err := db.CreateProject(p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	got, err := db.FindProject(p.ID)
	if err != nil {
		t.Fatalf("find project: %v", err)
	}
	if got.ComposeYAML != "" {
		t.Errorf("expected empty ComposeYAML on a fresh project, got %q", got.ComposeYAML)
	}
	if got.ExposedService != "" {
		t.Errorf("expected empty ExposedService on a fresh project, got %q", got.ExposedService)
	}

	got.ComposeYAML = "services:\n  app:\n    image: tamga-project-1\n"
	got.ExposedService = "app"
	if err := db.UpdateProject(got); err != nil {
		t.Fatalf("update project: %v", err)
	}

	updated, err := db.FindProject(p.ID)
	if err != nil {
		t.Fatalf("find project after update: %v", err)
	}
	if updated.ComposeYAML != got.ComposeYAML {
		t.Errorf("ComposeYAML round-trip mismatch: got %q, want %q", updated.ComposeYAML, got.ComposeYAML)
	}
	if updated.ExposedService != "app" {
		t.Errorf("ExposedService round-trip mismatch: got %q, want %q", updated.ExposedService, "app")
	}

	// ListProjects must scan the same columns without error too.
	list, err := db.ListProjects()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(list) != 1 || list[0].ComposeYAML != got.ComposeYAML {
		t.Fatalf("unexpected list result: %+v", list)
	}
}

// TestCreateProjectPersistsComposeFields is the regression guard for the
// FEAT-029 fix: CreateProject must BIND compose_yaml/exposed_service from the
// project (a compose project supplies them at create time), not hardcode ''.
// Before the fix the INSERT dropped these values and they only reappeared
// once deploy()'s async UpdateProject ran — a race with the detail-page load
// and a total loss across a backend restart.
func TestCreateProjectPersistsComposeFields(t *testing.T) {
	db := openTestDB(t)

	compose := "services:\n  web:\n    image: nginx:alpine\n    ports:\n      - \"8080:80\"\n  redis:\n    image: redis:7\n"
	p := &domain.Project{
		Name:           "compose-create",
		SourceType:     domain.SourceTypeCompose,
		Domain:         "compose-create.example.com",
		Status:         domain.ProjectStatusCreated,
		ComposeYAML:    compose,
		ExposedService: "web",
	}
	if err := db.CreateProject(p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	got, err := db.FindProject(p.ID)
	if err != nil {
		t.Fatalf("find project: %v", err)
	}
	if got.ComposeYAML != compose {
		t.Errorf("ComposeYAML not persisted on create: got %q, want %q", got.ComposeYAML, compose)
	}
	if got.ExposedService != "web" {
		t.Errorf("ExposedService not persisted on create: got %q, want %q", got.ExposedService, "web")
	}
}

// TestServiceContainerCRUD covers project_service_containers: listing an
// empty set, replacing the full set for a project (upsert-by-replace),
// re-replacing to drop a stale service, and cascade deletion when the
// parent project is deleted.
func TestServiceContainerCRUD(t *testing.T) {
	db := openTestDB(t)

	p := &domain.Project{
		Name:       "multi-service-app",
		SourceType: domain.SourceTypeRemote,
		RepoURL:    "https://example.invalid/org/repo.git",
		Branch:     "main",
		Status:     domain.ProjectStatusCreated,
	}
	if err := db.CreateProject(p); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Empty to start.
	containers, err := db.ListServiceContainers(p.ID)
	if err != nil {
		t.Fatalf("list service containers (empty): %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected 0 service containers, got %d", len(containers))
	}

	// Replace with two services.
	err = db.ReplaceServiceContainers(p.ID, []*domain.ServiceContainer{
		{ServiceName: "app", ContainerID: "abc123", ContainerName: "project-1-app", Status: "running"},
		{ServiceName: "db", ContainerID: "def456", ContainerName: "project-1-db", Status: "running"},
	})
	if err != nil {
		t.Fatalf("replace service containers: %v", err)
	}

	containers, err = db.ListServiceContainers(p.ID)
	if err != nil {
		t.Fatalf("list service containers: %v", err)
	}
	if len(containers) != 2 {
		t.Fatalf("expected 2 service containers, got %d", len(containers))
	}
	if containers[0].ServiceName != "app" || containers[1].ServiceName != "db" {
		t.Fatalf("unexpected service names/order: %+v, %+v", containers[0], containers[1])
	}
	if containers[0].ProjectID != p.ID {
		t.Errorf("expected ProjectID %d, got %d", p.ID, containers[0].ProjectID)
	}
	if containers[0].ContainerID != "abc123" || containers[0].ContainerName != "project-1-app" {
		t.Errorf("unexpected container fields: %+v", containers[0])
	}

	// Replacing again with a smaller set drops the stale row (a redeploy
	// that removed the "db" service).
	if err := db.ReplaceServiceContainers(p.ID, []*domain.ServiceContainer{
		{ServiceName: "app", ContainerID: "xyz789", ContainerName: "project-1-app", Status: "running"},
	}); err != nil {
		t.Fatalf("replace service containers (shrink): %v", err)
	}
	containers, err = db.ListServiceContainers(p.ID)
	if err != nil {
		t.Fatalf("list service containers after shrink: %v", err)
	}
	if len(containers) != 1 || containers[0].ContainerID != "xyz789" {
		t.Fatalf("expected exactly the replaced 'app' row, got %+v", containers)
	}

	// DeleteServiceContainersByProject explicitly clears the set without
	// deleting the project.
	if err := db.DeleteServiceContainersByProject(p.ID); err != nil {
		t.Fatalf("delete service containers by project: %v", err)
	}
	containers, err = db.ListServiceContainers(p.ID)
	if err != nil {
		t.Fatalf("list service containers after explicit delete: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected 0 service containers after explicit delete, got %d", len(containers))
	}
	if _, err := db.FindProject(p.ID); err != nil {
		t.Fatalf("expected project to still exist after clearing its service containers: %v", err)
	}

	// Cascade delete: the schema declares project_id ... ON DELETE CASCADE
	// (migration 000016, mirroring env_vars/000005), but like every other
	// child table in this codebase that isn't actually enforced unless
	// SQLite's per-connection "PRAGMA foreign_keys = ON" is set - the
	// existing ProjectService.Delete() doesn't rely on it either, it calls
	// DeleteEnvVarsByProject explicitly before DeleteProject. Turn the
	// pragma on here to prove the CASCADE clause itself is correct/would
	// take effect if enforcement is ever turned on, without changing
	// sqlite.Open()'s connection defaults for the rest of the app.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign_keys pragma: %v", err)
	}
	if err := db.ReplaceServiceContainers(p.ID, []*domain.ServiceContainer{
		{ServiceName: "app", ContainerID: "cascade1", ContainerName: "project-1-app", Status: "running"},
	}); err != nil {
		t.Fatalf("replace service containers before cascade check: %v", err)
	}
	if err := db.DeleteProject(p.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	containers, err = db.ListServiceContainers(p.ID)
	if err != nil {
		t.Fatalf("list service containers after project delete: %v", err)
	}
	if len(containers) != 0 {
		t.Fatalf("expected service containers cascade-deleted with project (foreign_keys=ON), got %d", len(containers))
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
