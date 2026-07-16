package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateEnvVar(ev *domain.EnvVar) error {
	res, err := db.Exec(
		"INSERT INTO env_vars (project_id, key, value) VALUES (?, ?, ?)",
		ev.ProjectID, ev.Key, ev.Value,
	)
	if err != nil {
		return fmt.Errorf("create env var: %w", err)
	}
	id, _ := res.LastInsertId()
	ev.ID = id
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

func (db *DB) DeleteEnvVar(projectID, id int64) error {
	_, err := db.Exec("DELETE FROM env_vars WHERE project_id = ? AND id = ?", projectID, id)
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

func (db *DB) UpsertServiceEnvVar(ev *domain.ServiceEnvVar) error {
	res, err := db.Exec(`INSERT INTO service_env_vars (project_id, service_name, key, value)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(project_id, service_name, key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`,
		ev.ProjectID, ev.ServiceName, ev.Key, ev.Value)
	if err != nil {
		return fmt.Errorf("upsert service env var: %w", err)
	}
	if ev.ID == 0 {
		if err := db.QueryRow("SELECT id, created_at, updated_at FROM service_env_vars WHERE project_id = ? AND service_name = ? AND key = ?", ev.ProjectID, ev.ServiceName, ev.Key).Scan(&ev.ID, &ev.CreatedAt, &ev.UpdatedAt); err != nil {
			return fmt.Errorf("find upserted service env var: %w", err)
		}
	}
	_ = res
	return nil
}

func (db *DB) ImportServiceEnvVars(vars []*domain.ServiceEnvVar) error {
	for _, ev := range vars {
		if _, err := db.Exec("INSERT INTO service_env_vars (project_id, service_name, key, value) VALUES (?, ?, ?, ?) ON CONFLICT(project_id, service_name, key) DO NOTHING", ev.ProjectID, ev.ServiceName, ev.Key, ev.Value); err != nil {
			return fmt.Errorf("import service env var: %w", err)
		}
	}
	return nil
}

func (db *DB) ListServiceEnvVars(projectID int64, serviceName string) ([]*domain.ServiceEnvVar, error) {
	rows, err := db.Query("SELECT id, project_id, service_name, key, value, created_at, updated_at FROM service_env_vars WHERE project_id = ? AND service_name = ? ORDER BY key", projectID, serviceName)
	if err != nil {
		return nil, fmt.Errorf("list service env vars: %w", err)
	}
	defer rows.Close()
	var vars []*domain.ServiceEnvVar
	for rows.Next() {
		v := &domain.ServiceEnvVar{}
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.ServiceName, &v.Key, &v.Value, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan service env var: %w", err)
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

func (db *DB) ListServiceEnvVarsByProject(projectID int64) ([]*domain.ServiceEnvVar, error) {
	rows, err := db.Query("SELECT id, project_id, service_name, key, value, created_at, updated_at FROM service_env_vars WHERE project_id = ? ORDER BY service_name, key", projectID)
	if err != nil {
		return nil, fmt.Errorf("list project service env vars: %w", err)
	}
	defer rows.Close()
	var vars []*domain.ServiceEnvVar
	for rows.Next() {
		v := &domain.ServiceEnvVar{}
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.ServiceName, &v.Key, &v.Value, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan service env var: %w", err)
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

func (db *DB) DeleteServiceEnvVar(projectID int64, serviceName string, id int64) error {
	_, err := db.Exec("DELETE FROM service_env_vars WHERE project_id = ? AND service_name = ? AND id = ?", projectID, serviceName, id)
	if err != nil {
		return fmt.Errorf("delete service env var: %w", err)
	}
	return nil
}

func (db *DB) DeleteServiceEnvVarsByProject(projectID int64) error {
	_, err := db.Exec("DELETE FROM service_env_vars WHERE project_id = ?", projectID)
	if err != nil {
		return fmt.Errorf("delete service env vars by project: %w", err)
	}
	return nil
}
