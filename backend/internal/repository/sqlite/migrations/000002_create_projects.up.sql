CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    repo_url TEXT NOT NULL,
    branch TEXT DEFAULT 'main',
    domain TEXT NOT NULL,
    status TEXT DEFAULT 'created',
    container_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
