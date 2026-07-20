package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateProject(project *domain.Project) error {
	result, err := db.Exec(`INSERT INTO projects
		(seal_id, name, source_type, repo_url, branch, compose_yaml, config_authority, status, config_revision, build_revision)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		project.SealID, project.Name, project.SourceType, project.RepoURL, project.Branch,
		project.ComposeYAML, project.ConfigAuthority, project.Status, project.ConfigRevision, project.BuildRevision)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	project.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) FindProject(sealID, projectID int64) (*domain.Project, error) {
	project := &domain.Project{}
	err := db.QueryRow(`SELECT id, seal_id, name, source_type, repo_url, branch, compose_yaml,
		config_authority, status, config_revision, build_revision, created_at, updated_at
		FROM projects WHERE seal_id=? AND id=?`, sealID, projectID).Scan(
		&project.ID, &project.SealID, &project.Name, &project.SourceType, &project.RepoURL,
		&project.Branch, &project.ComposeYAML, &project.ConfigAuthority, &project.Status,
		&project.ConfigRevision, &project.BuildRevision, &project.CreatedAt, &project.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	return project, nil
}

func (db *DB) ListProjects(sealID int64) ([]*domain.Project, error) {
	rows, err := db.Query(`SELECT id, seal_id, name, source_type, repo_url, branch, compose_yaml,
		config_authority, status, config_revision, build_revision, created_at, updated_at
		FROM projects WHERE seal_id=? ORDER BY created_at DESC`, sealID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		project := &domain.Project{}
		if err := rows.Scan(&project.ID, &project.SealID, &project.Name, &project.SourceType,
			&project.RepoURL, &project.Branch, &project.ComposeYAML, &project.ConfigAuthority,
			&project.Status, &project.ConfigRevision, &project.BuildRevision, &project.CreatedAt,
			&project.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (db *DB) UpdateProject(project *domain.Project) error {
	result, err := db.Exec(`UPDATE projects SET name=?, source_type=?, repo_url=?, branch=?,
		compose_yaml=?, config_authority=?, status=?, config_revision=?, build_revision=?,
		updated_at=CURRENT_TIMESTAMP WHERE id=? AND seal_id=?`,
		project.Name, project.SourceType, project.RepoURL, project.Branch, project.ComposeYAML,
		project.ConfigAuthority, project.Status, project.ConfigRevision, project.BuildRevision,
		project.ID, project.SealID)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated projects: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (db *DB) SetBuildStateIfRevision(sealID, projectID, configRevision, buildRevision int64, status domain.ProjectStatus) (bool, error) {
	result, err := db.Exec(`UPDATE projects SET status=?, build_revision=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND seal_id=? AND config_revision=?`, status, buildRevision, projectID, sealID, configRevision)
	if err != nil {
		return false, fmt.Errorf("set build state: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("set build state rows affected: %w", err)
	}
	return affected == 1, nil
}

func (db *DB) DeleteProject(sealID, projectID int64) error {
	result, err := db.Exec("DELETE FROM projects WHERE id=? AND seal_id=?", projectID, sealID)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted projects: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
