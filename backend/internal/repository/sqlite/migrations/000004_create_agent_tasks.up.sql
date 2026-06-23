CREATE TABLE IF NOT EXISTS agent_tasks (
    id TEXT PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    response TEXT,
    diff TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);
