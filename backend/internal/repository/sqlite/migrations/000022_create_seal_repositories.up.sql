CREATE TABLE IF NOT EXISTS seal_repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    remote_url TEXT NOT NULL,
    branch TEXT NOT NULL DEFAULT 'main',
    workspace_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    error_summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(seal_id, workspace_path)
);
