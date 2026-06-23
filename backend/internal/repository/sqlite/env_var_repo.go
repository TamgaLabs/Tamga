package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateEnvVar(ev *domain.EnvVar) error {
	_, err := db.Exec(
		"INSERT INTO env_vars (project_id, key, value) VALUES (?, ?, ?)",
		ev.ProjectID, ev.Key, ev.Value,
	)
	if err != nil {
		return fmt.Errorf("create env var: %w", err)
	}
	return nil
}

func (db *DB) ListEnvVars(projectID int64) ([]*domain.EnvVar, error) {
	rows, err := db.Query(
		"SELECT id, project_id, key, value, created_at, updated_at FROM env_vars WHERE project_id = ? ORDER BY key",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list env vars: %w", err)
	}
	defer rows.Close()

	var vars []*domain.EnvVar
	for rows.Next() {
		v := &domain.EnvVar{}
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Key, &v.Value, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan env var: %w", err)
		}
		vars = append(vars, v)
	}
	return vars, nil
}

func (db *DB) DeleteEnvVar(id int64) error {
	_, err := db.Exec("DELETE FROM env_vars WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete env var: %w", err)
	}
	return nil
}

func (db *DB) DeleteEnvVarsByProject(projectID int64) error {
	_, err := db.Exec("DELETE FROM env_vars WHERE project_id = ?", projectID)
	if err != nil {
		return fmt.Errorf("delete env vars by project: %w", err)
	}
	return nil
}
