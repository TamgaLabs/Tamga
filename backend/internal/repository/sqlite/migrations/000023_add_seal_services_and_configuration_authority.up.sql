ALTER TABLE seals ADD COLUMN config_authority TEXT NOT NULL DEFAULT 'direct' CHECK (config_authority IN ('generated', 'direct'));

CREATE TABLE seal_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals(id) ON DELETE CASCADE,
    repository_id INTEGER NOT NULL REFERENCES seal_repositories(id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    build_context TEXT NOT NULL,
    internal_port INTEGER NOT NULL CHECK (internal_port BETWEEN 1 AND 65535),
    dependencies_json TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(seal_id, name)
);
