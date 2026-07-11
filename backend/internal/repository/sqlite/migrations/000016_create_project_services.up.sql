ALTER TABLE projects ADD COLUMN compose_yaml TEXT;
ALTER TABLE projects ADD COLUMN exposed_service TEXT;

CREATE TABLE IF NOT EXISTS project_service_containers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_name TEXT NOT NULL,
    container_id TEXT NOT NULL DEFAULT '',
    container_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'created',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, service_name)
);
