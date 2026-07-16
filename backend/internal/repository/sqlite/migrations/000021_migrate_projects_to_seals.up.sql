BEGIN IMMEDIATE;

CREATE TABLE seals_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    source_type TEXT NOT NULL DEFAULT 'remote',
    repo_url TEXT NOT NULL DEFAULT '',
    branch TEXT DEFAULT 'main',
    domain TEXT NOT NULL DEFAULT '',
    status TEXT DEFAULT 'created',
    container_id TEXT NOT NULL DEFAULT '',
    compose_yaml TEXT,
    exposed_service TEXT,
    config_revision INTEGER NOT NULL DEFAULT 0,
    build_revision INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO seals_new (id, name, source_type, repo_url, branch, domain, status, container_id, compose_yaml, exposed_service, config_revision, build_revision, created_at, updated_at)
SELECT id, name, source_type, repo_url, branch, domain, status, COALESCE(container_id, ''), compose_yaml, exposed_service, config_revision, build_revision, created_at, updated_at FROM projects;

CREATE TABLE deployments_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals_new(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'pending', commit_sha TEXT, logs TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO deployments_new SELECT id, project_id, status, commit_sha, logs, created_at, updated_at FROM deployments;

CREATE TABLE env_vars_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals_new(id) ON DELETE CASCADE,
    key TEXT NOT NULL, value TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(seal_id, key)
);
INSERT INTO env_vars_new SELECT id, project_id, key, value, created_at, updated_at FROM env_vars;

CREATE TABLE agent_tasks_new (
    id TEXT PRIMARY KEY,
    seal_id INTEGER NOT NULL REFERENCES seals_new(id) ON DELETE CASCADE,
    message TEXT NOT NULL, status TEXT DEFAULT 'pending', response TEXT, diff TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP, completed_at DATETIME, session_id TEXT REFERENCES agent_sessions(id) ON DELETE SET NULL
);
INSERT INTO agent_tasks_new SELECT id, project_id, message, status, response, diff, created_at, completed_at, session_id FROM agent_tasks;

CREATE TABLE agent_sessions_new (
    id TEXT PRIMARY KEY,
    seal_id INTEGER NOT NULL DEFAULT 0,
    dir_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT 'New Chat',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO agent_sessions_new SELECT id, project_id, dir_id, name, created_at, updated_at FROM agent_sessions;

CREATE TABLE seal_service_containers_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals_new(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL, container_id TEXT NOT NULL DEFAULT '', container_name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'created',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, UNIQUE(seal_id, service_name)
);
INSERT INTO seal_service_containers_new SELECT id, project_id, service_name, container_id, container_name, status, created_at FROM project_service_containers;

CREATE TABLE metric_samples_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL DEFAULT 0,
    resolution TEXT NOT NULL CHECK (resolution IN ('minute', 'hour', 'day')), bucket_start INTEGER NOT NULL,
    count_2xx INTEGER NOT NULL DEFAULT 0, count_3xx INTEGER NOT NULL DEFAULT 0, count_4xx INTEGER NOT NULL DEFAULT 0, count_5xx INTEGER NOT NULL DEFAULT 0,
    bytes_in INTEGER NOT NULL DEFAULT 0, bytes_out INTEGER NOT NULL DEFAULT 0, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(seal_id, resolution, bucket_start)
);
INSERT INTO metric_samples_new SELECT id, project_id, resolution, bucket_start, count_2xx, count_3xx, count_4xx, count_5xx, bytes_in, bytes_out, created_at FROM metric_samples;

CREATE TABLE metric_latency_buckets_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL DEFAULT 0,
    resolution TEXT NOT NULL CHECK (resolution IN ('minute', 'hour', 'day')), bucket_start INTEGER NOT NULL, le TEXT NOT NULL, count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, UNIQUE(seal_id, resolution, bucket_start, le)
);
INSERT INTO metric_latency_buckets_new SELECT id, project_id, resolution, bucket_start, le, count, created_at FROM metric_latency_buckets;

CREATE TABLE seal_sources_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals_new(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL, remote_url TEXT NOT NULL DEFAULT '', branch TEXT NOT NULL DEFAULT 'main', workspace_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending', error_summary TEXT NOT NULL DEFAULT '', created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(seal_id, workspace_path)
);
INSERT INTO seal_sources_new SELECT id, project_id, display_name, remote_url, branch, workspace_path, status, error_summary, created_at, updated_at FROM project_sources;

CREATE TABLE seal_routes_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals_new(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL, domain TEXT NOT NULL COLLATE NOCASE UNIQUE, UNIQUE(seal_id, service_name, domain)
);
INSERT INTO seal_routes_new SELECT id, project_id, service_name, domain FROM project_routes;

CREATE TABLE service_env_vars_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals_new(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(seal_id, service_name, key)
);
INSERT INTO service_env_vars_new SELECT id, project_id, service_name, key, value, created_at, updated_at FROM service_env_vars;

DROP TABLE agent_tasks;
DROP TABLE agent_sessions;
DROP TABLE deployments;
DROP TABLE env_vars;
DROP TABLE project_service_containers;
DROP TABLE metric_samples;
DROP TABLE metric_latency_buckets;
DROP TABLE project_sources;
DROP TABLE project_routes;
DROP TABLE service_env_vars;
DROP TABLE projects;

ALTER TABLE seals_new RENAME TO seals;
ALTER TABLE deployments_new RENAME TO deployments;
ALTER TABLE env_vars_new RENAME TO env_vars;
ALTER TABLE agent_sessions_new RENAME TO agent_sessions;
ALTER TABLE agent_tasks_new RENAME TO agent_tasks;
ALTER TABLE seal_service_containers_new RENAME TO seal_service_containers;
ALTER TABLE metric_samples_new RENAME TO metric_samples;
ALTER TABLE metric_latency_buckets_new RENAME TO metric_latency_buckets;
ALTER TABLE seal_sources_new RENAME TO seal_sources;
ALTER TABLE seal_routes_new RENAME TO seal_routes;
ALTER TABLE service_env_vars_new RENAME TO service_env_vars;

COMMIT;
