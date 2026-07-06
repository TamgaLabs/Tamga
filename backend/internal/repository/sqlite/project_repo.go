package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateProject(p *domain.Project) error {
	// container_id is set explicitly to '' (rather than left to default to
	// NULL) so FindProject/ListProjects - which scan straight into a plain
	// string field - never hit a NULL-into-string scan error for a project
	// that hasn't been deployed yet.
	res, err := db.Exec(
		"INSERT INTO projects (name, source_type, repo_url, branch, domain, status, agent_provider_id, container_id) VALUES (?, ?, ?, ?, ?, ?, ?, '')",
		p.Name, p.SourceType, p.RepoURL, p.Branch, p.Domain, p.Status, p.AgentProviderID,
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
		"SELECT id, name, source_type, repo_url, branch, domain, status, container_id, agent_provider_id, created_at, updated_at FROM projects WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.SourceType, &p.RepoURL, &p.Branch, &p.Domain, &p.Status, &p.ContainerID, &p.AgentProviderID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	return p, nil
}

func (db *DB) ListProjects() ([]*domain.Project, error) {
	rows, err := db.Query("SELECT id, name, source_type, repo_url, branch, domain, status, container_id, agent_provider_id, created_at, updated_at FROM projects ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*domain.Project
	for rows.Next() {
		p := &domain.Project{}
		if err := rows.Scan(&p.ID, &p.Name, &p.SourceType, &p.RepoURL, &p.Branch, &p.Domain, &p.Status, &p.ContainerID, &p.AgentProviderID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (db *DB) UpdateProject(p *domain.Project) error {
	_, err := db.Exec(
		"UPDATE projects SET name=?, source_type=?, repo_url=?, branch=?, domain=?, status=?, container_id=?, agent_provider_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		p.Name, p.SourceType, p.RepoURL, p.Branch, p.Domain, p.Status, p.ContainerID, p.AgentProviderID, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

func (db *DB) DeleteProject(id int64) error {
	_, err := db.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}
