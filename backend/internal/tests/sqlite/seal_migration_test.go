package sqlite_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

func TestProjectScopedDataMigratesToSealStorage(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	legacySchema := `
CREATE TABLE schema_migrations (filename TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE projects (id INTEGER PRIMARY KEY, name TEXT NOT NULL, source_type TEXT NOT NULL, repo_url TEXT NOT NULL, branch TEXT, domain TEXT NOT NULL, status TEXT, container_id TEXT, compose_yaml TEXT, exposed_service TEXT, config_revision INTEGER NOT NULL DEFAULT 0, build_revision INTEGER NOT NULL DEFAULT 0, created_at DATETIME, updated_at DATETIME);
CREATE TABLE deployments (id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, status TEXT, commit_sha TEXT, logs TEXT, created_at DATETIME, updated_at DATETIME);
CREATE TABLE env_vars (id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, key TEXT, value TEXT, created_at DATETIME, updated_at DATETIME);
CREATE TABLE agent_sessions (id TEXT PRIMARY KEY, project_id INTEGER NOT NULL, dir_id TEXT, name TEXT, created_at DATETIME, updated_at DATETIME);
CREATE TABLE agent_tasks (id TEXT PRIMARY KEY, project_id INTEGER NOT NULL, message TEXT, status TEXT, response TEXT, diff TEXT, created_at DATETIME, completed_at DATETIME, session_id TEXT);
CREATE TABLE project_service_containers (id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, service_name TEXT, container_id TEXT, container_name TEXT, status TEXT, created_at DATETIME);
CREATE TABLE metric_samples (id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, resolution TEXT, bucket_start INTEGER, count_2xx INTEGER, count_3xx INTEGER, count_4xx INTEGER, count_5xx INTEGER, bytes_in INTEGER, bytes_out INTEGER, created_at DATETIME);
CREATE TABLE metric_latency_buckets (id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, resolution TEXT, bucket_start INTEGER, le TEXT, count INTEGER, created_at DATETIME);
CREATE TABLE project_sources (id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, display_name TEXT, remote_url TEXT, branch TEXT, workspace_path TEXT, status TEXT, error_summary TEXT, created_at DATETIME, updated_at DATETIME);
CREATE TABLE project_routes (id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, service_name TEXT, domain TEXT);
CREATE TABLE service_env_vars (id INTEGER PRIMARY KEY, project_id INTEGER NOT NULL, service_name TEXT, key TEXT, value TEXT, created_at DATETIME, updated_at DATETIME);
INSERT INTO projects VALUES (42, 'legacy', 'remote', 'https://example.test/legacy.git', 'main', 'legacy.test', 'running', '', 'services: {}', '', 3, 5, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO deployments VALUES (1, 42, 'success', 'abc', 'ok', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO env_vars VALUES (1, 42, 'KEY', 'value', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO agent_sessions VALUES ('session', 42, '', 'Legacy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO agent_tasks VALUES ('task', 42, 'hello', 'pending', '', '', CURRENT_TIMESTAMP, NULL, 'session');
INSERT INTO project_service_containers VALUES (1, 42, 'web', 'container', 'legacy-web', 'running', CURRENT_TIMESTAMP);
INSERT INTO metric_samples VALUES (1, 42, 'minute', 100, 1, 0, 0, 0, 2, 3, CURRENT_TIMESTAMP);
INSERT INTO metric_samples VALUES (2, 0, 'minute', 100, 4, 0, 0, 0, 5, 6, CURRENT_TIMESTAMP);
INSERT INTO metric_latency_buckets VALUES (1, 42, 'minute', 100, '1', 7, CURRENT_TIMESTAMP);
INSERT INTO project_sources VALUES (1, 42, 'primary', 'https://example.test/legacy.git', 'main', '.', 'ready', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
INSERT INTO project_routes VALUES (1, 42, 'web', 'legacy.test');
INSERT INTO service_env_vars VALUES (1, 42, 'web', 'KEY', 'value', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);`
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	for i := 1; i <= 20; i++ {
		if _, err := db.Exec("INSERT INTO schema_migrations (filename) VALUES (?)", fmt.Sprintf("%06d_placeholder.up.sql", i)); err != nil {
			t.Fatalf("mark legacy migration %d: %v", i, err)
		}
	}
	// Match the embedded filenames so only 000021 is pending.
	if _, err := db.Exec("DELETE FROM schema_migrations"); err != nil {
		t.Fatalf("clear placeholders: %v", err)
	}
	for _, name := range []string{"000001_create_users.up.sql", "000002_create_projects.up.sql", "000003_create_deployments.up.sql", "000004_create_agent_tasks.up.sql", "000005_create_env_vars.up.sql", "000006_add_source_type.up.sql", "000007_create_agent_sessions.up.sql", "000008_create_agent_providers.up.sql", "000009_drop_agent_provider_obsolete_fields.up.sql", "000010_create_egress_whitelist.up.sql", "000011_create_resource_limits.up.sql", "000012_create_git_credential.up.sql", "000013_drop_agent_providers.up.sql", "000014_create_egress_blacklist_and_settings.up.sql", "000015_create_idle_timeout_settings.up.sql", "000016_create_project_services.up.sql", "000017_create_metrics_timeseries.up.sql", "000018_create_project_sources.up.sql", "000019_create_project_builds_and_routes.up.sql", "000020_create_service_env_vars.up.sql"} {
		if _, err := db.Exec("INSERT INTO schema_migrations (filename) VALUES (?)", name); err != nil {
			t.Fatalf("mark %s applied: %v", name, err)
		}
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate legacy project data: %v", err)
	}
	seal, err := db.FindSeal(42)
	if err != nil || seal.Name != "legacy" || seal.ConfigRevision != 3 || seal.BuildRevision != 5 {
		t.Fatalf("legacy seal is not reachable after migration: seal=%+v err=%v", seal, err)
	}
	for _, query := range []string{
		"SELECT COUNT(*) FROM deployments WHERE seal_id = 42",
		"SELECT COUNT(*) FROM env_vars WHERE seal_id = 42",
		"SELECT COUNT(*) FROM agent_sessions WHERE seal_id = 42",
		"SELECT COUNT(*) FROM agent_tasks WHERE seal_id = 42",
		"SELECT COUNT(*) FROM seal_service_containers WHERE seal_id = 42",
		"SELECT COUNT(*) FROM metric_samples WHERE seal_id = 42",
		"SELECT COUNT(*) FROM metric_samples WHERE seal_id = 0",
		"SELECT COUNT(*) FROM metric_latency_buckets WHERE seal_id = 42",
		"SELECT COUNT(*) FROM seal_sources WHERE seal_id = 42",
		"SELECT COUNT(*) FROM seal_routes WHERE seal_id = 42",
		"SELECT COUNT(*) FROM service_env_vars WHERE seal_id = 42",
	} {
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatalf("ownership not preserved for %q: count=%d err=%v", query, count, err)
		}
	}
	var projectsTable int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'projects'").Scan(&projectsTable); err != nil || projectsTable != 0 {
		t.Fatalf("legacy projects table remains: count=%d err=%v", projectsTable, err)
	}
}
