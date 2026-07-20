package sqlite_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

func TestFreshSealProjectBaseline(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("open fresh database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate fresh database: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("re-migrate fresh database: %v", err)
	}

	for _, table := range []string{"seals", "projects", "services", "service_routes", "service_containers", "service_env_vars", "deployments", "project_topologies"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("expected %s table: count=%d err=%v", table, count, err)
		}
	}
	for _, table := range []string{"project_sources", "seal_sources", "seal_routes", "seal_services", "seal_repositories", "project_service_containers", "env_vars"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("legacy table remains: %s count=%d err=%v", table, count, err)
		}
	}

	if _, err := db.Exec("INSERT INTO seals (name) VALUES ('seal')"); err != nil {
		t.Fatalf("create seal: %v", err)
	}
	if _, err := db.Exec("INSERT INTO projects (seal_id, name) VALUES (1, 'project')"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := db.Exec("INSERT INTO services (project_id, name, internal_port) VALUES (1, 'web', 8080)"); err != nil {
		t.Fatalf("create service: %v", err)
	}
	if _, err := db.Exec("INSERT INTO service_routes (service_id, domain) VALUES (1, 'app.example.test')"); err != nil {
		t.Fatalf("create route: %v", err)
	}
	if _, err := db.Exec("INSERT INTO service_routes (service_id, domain) VALUES (1, 'APP.EXAMPLE.TEST')"); err == nil {
		t.Fatal("accepted non-normalized route domain")
	}
	if _, err := db.Exec("INSERT INTO service_routes (service_id, domain) VALUES (1, 'app.example.test')"); err == nil {
		t.Fatal("accepted globally duplicated route domain")
	}
	if _, err := db.Exec("DELETE FROM seals WHERE id = 1"); err != nil {
		t.Fatalf("delete seal: %v", err)
	}
	var services int
	if err := db.QueryRow("SELECT COUNT(*) FROM services").Scan(&services); err != nil || services != 0 {
		t.Fatalf("seal cascade left services: count=%d err=%v", services, err)
	}
}

func TestFreshBaselinePersistsGlobalMetrics(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("open fresh database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate fresh database: %v", err)
	}

	bucketStart := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	if err := db.InsertMetricSamples([]*domain.MetricSample{{
		ProjectID: domain.GlobalProjectID, Resolution: domain.MetricResolutionMinute, BucketStart: bucketStart, Count2xx: 1,
	}}); err != nil {
		t.Fatalf("persist global metric sample on fresh database: %v", err)
	}
	if err := db.InsertMetricLatencyBuckets([]*domain.MetricLatencyBucket{{
		ProjectID: domain.GlobalProjectID, Resolution: domain.MetricResolutionMinute, BucketStart: bucketStart, Le: "+Inf", Count: 1,
	}}); err != nil {
		t.Fatalf("persist global metric latency bucket on fresh database: %v", err)
	}
}

func TestFreshBaselineAgentPersistenceIsProjectOwned(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("open fresh database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate fresh database: %v", err)
	}

	if _, err := db.Exec("INSERT INTO seals (id, name) VALUES (1, 'tenant')"); err != nil {
		t.Fatalf("create tenant seal: %v", err)
	}
	if _, err := db.Exec("INSERT INTO projects (id, seal_id, name) VALUES (1, 1, 'application')"); err != nil {
		t.Fatalf("create tenant project: %v", err)
	}
	if _, err := db.Exec("INSERT INTO agent_sessions (id, project_id) VALUES ('session-1', 1)"); err != nil {
		t.Fatalf("create project-owned agent session: %v", err)
	}
	if _, err := db.Exec("INSERT INTO agent_tasks (id, project_id, session_id, message) VALUES ('task-1', 1, 'session-1', 'inspect the project')"); err != nil {
		t.Fatalf("create project-owned agent task: %v", err)
	}
	if _, err := db.Exec("INSERT INTO agent_sessions (id, project_id) VALUES ('orphan-session', 99)"); err == nil {
		t.Fatal("accepted agent session without a project owner")
	}
	if _, err := db.Exec("INSERT INTO agent_tasks (id, project_id, message) VALUES ('orphan-task', 99, 'inspect')"); err == nil {
		t.Fatal("accepted agent task without a project owner")
	}

	if _, err := db.Exec("DELETE FROM projects WHERE id = 1"); err != nil {
		t.Fatalf("delete tenant project: %v", err)
	}
	for _, table := range []string{"agent_sessions", "agent_tasks"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("project cascade left %s rows: count=%d err=%v", table, count, err)
		}
	}
}
