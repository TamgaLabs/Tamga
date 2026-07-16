package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateProject(p *domain.Project) error {
	// container_id is set explicitly to '' (rather than left to default to
	// NULL) so FindProject/ListProjects - which scan straight into plain
	// string fields - never hit a NULL-into-string scan error for a project
	// that hasn't been deployed yet. compose_yaml/exposed_service ARE bound
	// from the request (a compose project supplies them at create time); they
	// stay NULL-able at the column level (migration 000016) only so
	// pre-existing rows don't need a backfill, and the SELECTs below COALESCE
	// them to '' so a NULL legacy row and an empty new row scan identically.
	res, err := db.Exec(
		"INSERT INTO seals (name, source_type, repo_url, branch, domain, status, container_id, compose_yaml, exposed_service) VALUES (?, ?, ?, ?, ?, ?, '', ?, ?)",
		p.Name, p.SourceType, p.RepoURL, p.Branch, p.Domain, p.Status, p.ComposeYAML, p.ExposedService,
	)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	id, _ := res.LastInsertId()
	p.ID = id
	return nil
}

func (db *DB) FindProject(id int64) (*domain.Project, error) {
	p := &domain.Project{}
	err := db.QueryRow(
		"SELECT id, name, source_type, repo_url, branch, domain, status, container_id, COALESCE(compose_yaml, ''), COALESCE(exposed_service, ''), config_revision, build_revision, created_at, updated_at FROM seals WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.SourceType, &p.RepoURL, &p.Branch, &p.Domain, &p.Status, &p.ContainerID, &p.ComposeYAML, &p.ExposedService, &p.ConfigRevision, &p.BuildRevision, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	return p, nil
}

func (db *DB) ListProjects() ([]*domain.Project, error) {
	rows, err := db.Query("SELECT id, name, source_type, repo_url, branch, domain, status, container_id, COALESCE(compose_yaml, ''), COALESCE(exposed_service, ''), config_revision, build_revision, created_at, updated_at FROM seals ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p := &domain.Project{}
		if err := rows.Scan(&p.ID, &p.Name, &p.SourceType, &p.RepoURL, &p.Branch, &p.Domain, &p.Status, &p.ContainerID, &p.ComposeYAML, &p.ExposedService, &p.ConfigRevision, &p.BuildRevision, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (db *DB) UpdateProject(p *domain.Project) error {
	_, err := db.Exec(
		"UPDATE seals SET name=?, source_type=?, repo_url=?, branch=?, domain=?, status=?, container_id=?, compose_yaml=?, exposed_service=?, config_revision=?, build_revision=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		p.Name, p.SourceType, p.RepoURL, p.Branch, p.Domain, p.Status, p.ContainerID, p.ComposeYAML, p.ExposedService, p.ConfigRevision, p.BuildRevision, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

// SetBuildStateIfRevision changes only build-owned fields when the project is
// still at the configuration revision captured by Build.  It deliberately
// avoids writing the rest of a stale Project struct back over a concurrent
// configuration/source update.
func (db *DB) SetBuildStateIfRevision(id, configRevision, buildRevision int64, status domain.ProjectStatus) (bool, error) {
	result, err := db.Exec(
		"UPDATE seals SET status=?, build_revision=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND config_revision=?",
		status, buildRevision, id, configRevision,
	)
	if err != nil {
		return false, fmt.Errorf("set build state: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("set build state rows affected: %w", err)
	}
	return affected == 1, nil
}

func (db *DB) DeleteProject(id int64) error {
	_, err := db.Exec("DELETE FROM seals WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}
