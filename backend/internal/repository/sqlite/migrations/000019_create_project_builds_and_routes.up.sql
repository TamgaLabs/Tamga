ALTER TABLE projects ADD COLUMN config_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN build_revision INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS project_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    domain TEXT NOT NULL COLLATE NOCASE UNIQUE,
    UNIQUE(project_id, service_name, domain)
);
