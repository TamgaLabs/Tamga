CREATE TABLE IF NOT EXISTS project_sources (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    remote_url TEXT NOT NULL DEFAULT '',
    branch TEXT NOT NULL DEFAULT 'main',
    workspace_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    error_summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, workspace_path)
);

-- Keep legacy project rows readable while making their original remote the
-- primary source. Compose/local projects have no remote source to backfill.
INSERT INTO project_sources (project_id, display_name, remote_url, branch, workspace_path, status)
SELECT id, name, repo_url, COALESCE(NULLIF(branch, ''), 'main'), '.', 'ready'
FROM projects
WHERE COALESCE(repo_url, '') <> ''
  AND NOT EXISTS (
      SELECT 1 FROM project_sources ps
      WHERE ps.project_id = projects.id AND ps.workspace_path = '.'
  );
