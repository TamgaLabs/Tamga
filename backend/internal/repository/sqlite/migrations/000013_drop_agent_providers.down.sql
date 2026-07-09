CREATE TABLE IF NOT EXISTS agent_providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'docker',
    image TEXT NOT NULL DEFAULT '',
    env TEXT NOT NULL DEFAULT '{}',
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE projects ADD COLUMN agent_provider_id TEXT REFERENCES agent_providers(id);

INSERT OR IGNORE INTO agent_providers (id, name, type, image, is_default) VALUES
('builtin-opencode', 'Opencode (Built-in)', 'docker', 'tamga-agent', 1);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    key_enc TEXT NOT NULL,
    label TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
