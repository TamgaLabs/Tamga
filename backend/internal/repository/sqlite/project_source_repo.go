package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateProjectSource(source *domain.ProjectSource) error {
	result, err := db.Exec(`INSERT INTO seal_sources
	        (seal_id, display_name, remote_url, branch, workspace_path, status, error_summary)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		source.ProjectID, source.DisplayName, source.RemoteURL, source.Branch,
		source.WorkspacePath, source.Status, source.ErrorSummary)
	if err != nil {
		return fmt.Errorf("create project source: %w", err)
	}
	source.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) FindProjectSource(projectID, sourceID int64) (*domain.ProjectSource, error) {
	source := &domain.ProjectSource{}
	err := db.QueryRow(`SELECT id, seal_id, display_name, remote_url, branch,
        workspace_path, status, error_summary, created_at, updated_at
        FROM seal_sources WHERE seal_id = ? AND id = ?`, projectID, sourceID).
		Scan(&source.ID, &source.ProjectID, &source.DisplayName, &source.RemoteURL,
			&source.Branch, &source.WorkspacePath, &source.Status, &source.ErrorSummary,
			&source.CreatedAt, &source.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find project source: %w", err)
	}
	return source, nil
}

func (db *DB) ListProjectSources(projectID int64) ([]*domain.ProjectSource, error) {
	rows, err := db.Query(`SELECT id, seal_id, display_name, remote_url, branch,
        workspace_path, status, error_summary, created_at, updated_at
        FROM seal_sources WHERE seal_id = ? ORDER BY id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project sources: %w", err)
	}
	defer rows.Close()
	var sources []*domain.ProjectSource
	for rows.Next() {
		source := &domain.ProjectSource{}
		if err := rows.Scan(&source.ID, &source.ProjectID, &source.DisplayName,
			&source.RemoteURL, &source.Branch, &source.WorkspacePath, &source.Status,
			&source.ErrorSummary, &source.CreatedAt, &source.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project source: %w", err)
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (db *DB) UpdateProjectSource(source *domain.ProjectSource) error {
	_, err := db.Exec(`UPDATE seal_sources SET display_name=?, remote_url=?, branch=?,
	        workspace_path=?, status=?, error_summary=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND seal_id=?`,
		source.DisplayName, source.RemoteURL, source.Branch, source.WorkspacePath,
		source.Status, source.ErrorSummary, source.ID, source.ProjectID)
	if err != nil {
		return fmt.Errorf("update project source: %w", err)
	}
	return nil
}

func (db *DB) DeleteProjectSource(projectID, sourceID int64) error {
	_, err := db.Exec("DELETE FROM seal_sources WHERE seal_id = ? AND id = ?", projectID, sourceID)
	if err != nil {
		return fmt.Errorf("delete project source: %w", err)
	}
	return nil
}
