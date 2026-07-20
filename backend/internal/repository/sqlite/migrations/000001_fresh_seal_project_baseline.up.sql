-- Destructive fresh-install baseline. Existing numbered-migration databases are
-- intentionally unsupported; this schema models Seal → Project → Service → ServiceRoute.

CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE seals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    source_type TEXT NOT NULL DEFAULT 'empty',
    repo_url TEXT NOT NULL DEFAULT '',
    branch TEXT NOT NULL DEFAULT 'main',
    compose_yaml TEXT NOT NULL DEFAULT '',
    config_authority TEXT NOT NULL DEFAULT 'direct' CHECK (config_authority IN ('generated', 'direct')),
    status TEXT NOT NULL DEFAULT 'created',
    config_revision INTEGER NOT NULL DEFAULT 0,
    build_revision INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(seal_id, name)
);

-- Global/core Traefik traffic is project-owned too. ID 0 is a durable
-- system project rather than a fixture-only sentinel, so metric FKs remain
-- valid on a freshly migrated production database.
INSERT INTO seals (id, name) VALUES (0, 'tamga-system');
INSERT INTO projects (id, seal_id, name) VALUES (0, 0, 'tamga-system');

CREATE TABLE services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    build_context TEXT NOT NULL DEFAULT '.',
    internal_port INTEGER NOT NULL CHECK (internal_port BETWEEN 1 AND 65535),
    dependencies_json TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, name)
);

CREATE TABLE service_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    domain TEXT NOT NULL COLLATE BINARY CHECK (domain = lower(trim(domain))),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(domain)
);

CREATE TABLE service_containers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    container_id TEXT NOT NULL DEFAULT '',
    container_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'created',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service_id, container_id)
);

CREATE TABLE service_env_vars (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service_id, key)
);

CREATE TABLE deployments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    commit_sha TEXT,
    logs TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE project_topologies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    topology_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id)
);

CREATE TABLE agent_sessions (
    id TEXT PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    dir_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT 'New Chat',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE agent_tasks (
    id TEXT PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    session_id TEXT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    message TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    response TEXT,
    diff TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

CREATE TABLE metric_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    resolution TEXT NOT NULL CHECK (resolution IN ('minute', 'hour', 'day')),
    bucket_start INTEGER NOT NULL,
    count_2xx INTEGER NOT NULL DEFAULT 0,
    count_3xx INTEGER NOT NULL DEFAULT 0,
    count_4xx INTEGER NOT NULL DEFAULT 0,
    count_5xx INTEGER NOT NULL DEFAULT 0,
    bytes_in INTEGER NOT NULL DEFAULT 0,
    bytes_out INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, resolution, bucket_start)
);

CREATE TABLE metric_latency_buckets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    resolution TEXT NOT NULL CHECK (resolution IN ('minute', 'hour', 'day')),
    bucket_start INTEGER NOT NULL,
    le TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, resolution, bucket_start, le)
);

CREATE TABLE egress_whitelist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO egress_whitelist (domain) VALUES
    ('api.anthropic.com'),
    ('api.openai.com'),
    ('generativelanguage.googleapis.com');

CREATE TABLE egress_blacklist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE egress_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    mode TEXT NOT NULL DEFAULT 'open'
);
INSERT INTO egress_settings (id, mode) VALUES (1, 'open');

CREATE TABLE resource_limits (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    memory_bytes INTEGER NOT NULL,
    nano_cpus INTEGER NOT NULL
);
INSERT INTO resource_limits (id, memory_bytes, nano_cpus) VALUES (1, 1073741824, 1000000000);

CREATE TABLE git_credential (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    provider TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    token_enc TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO git_credential (id) VALUES (1);

CREATE TABLE idle_timeout_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    timeout_seconds INTEGER NOT NULL DEFAULT 0
);
INSERT INTO idle_timeout_settings (id, timeout_seconds) VALUES (1, 0);
