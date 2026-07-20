package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) UpsertServiceEnvVar(projectID int64, ev *domain.ServiceEnvVar) error {
	result, err := db.Exec(`INSERT INTO service_env_vars (service_id, key, value)
		SELECT s.id, ?, ? FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=? AND s.id=?
		ON CONFLICT(service_id, key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`,
		ev.Key, ev.Value, projectID, ev.ServiceID)
	if err != nil {
		return fmt.Errorf("upsert service env var: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("check upserted service env var ownership: %w", err)
	} else if affected == 0 {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(
			SELECT 1 FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=? AND s.id=?)`, projectID, ev.ServiceID).Scan(&exists); err != nil {
			return fmt.Errorf("check upserted service ownership: %w", err)
		}
		if !exists {
			return fmt.Errorf("service %d is not owned by project %d", ev.ServiceID, projectID)
		}
	}
	if ev.ID == 0 {
		if err := db.QueryRow(`SELECT sev.id, sev.created_at, sev.updated_at
			FROM service_env_vars sev
			JOIN services s ON s.id=sev.service_id
			JOIN projects p ON p.id=s.project_id
			WHERE p.id=? AND s.id=? AND sev.key=?`, projectID, ev.ServiceID, ev.Key).Scan(&ev.ID, &ev.CreatedAt, &ev.UpdatedAt); err != nil {
			return fmt.Errorf("find upserted service env var: %w", err)
		}
	}
	return nil
}

func (db *DB) ImportServiceEnvVars(projectID int64, vars []*domain.ServiceEnvVar) error {
	for _, ev := range vars {
		result, err := db.Exec(`INSERT INTO service_env_vars (service_id, key, value)
			SELECT s.id, ?, ? FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=? AND s.id=?
			ON CONFLICT(service_id, key) DO NOTHING`, ev.Key, ev.Value, projectID, ev.ServiceID)
		if err != nil {
			return fmt.Errorf("import service env var: %w", err)
		}
		if affected, err := result.RowsAffected(); err != nil {
			return fmt.Errorf("check imported service env var ownership: %w", err)
		} else if affected == 0 {
			var exists bool
			if err := db.QueryRow(`SELECT EXISTS(
				SELECT 1 FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=? AND s.id=?)`, projectID, ev.ServiceID).Scan(&exists); err != nil {
				return fmt.Errorf("check imported service ownership: %w", err)
			}
			if !exists {
				return fmt.Errorf("service %d is not owned by project %d", ev.ServiceID, projectID)
			}
		}
	}
	return nil
}

func (db *DB) ListServiceEnvVars(projectID, serviceID int64) ([]*domain.ServiceEnvVar, error) {
	rows, err := db.Query(`SELECT sev.id, sev.service_id, sev.key, sev.value, sev.created_at, sev.updated_at
		FROM service_env_vars sev
		JOIN services s ON s.id=sev.service_id
		JOIN projects p ON p.id=s.project_id
		WHERE p.id=? AND s.id=?
		ORDER BY sev.key`, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list service env vars: %w", err)
	}
	defer rows.Close()
	var vars []*domain.ServiceEnvVar
	for rows.Next() {
		v := &domain.ServiceEnvVar{}
		if err := rows.Scan(&v.ID, &v.ServiceID, &v.Key, &v.Value, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan service env var: %w", err)
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

func (db *DB) ListServiceEnvVarsByProject(projectID int64) ([]*domain.ServiceEnvVar, error) {
	rows, err := db.Query(`SELECT sev.id, sev.service_id, sev.key, sev.value, sev.created_at, sev.updated_at
		FROM service_env_vars sev
		JOIN services s ON s.id=sev.service_id
		JOIN projects p ON p.id=s.project_id
		WHERE p.id=?
		ORDER BY sev.service_id, sev.key`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project service env vars: %w", err)
	}
	defer rows.Close()
	var vars []*domain.ServiceEnvVar
	for rows.Next() {
		v := &domain.ServiceEnvVar{}
		if err := rows.Scan(&v.ID, &v.ServiceID, &v.Key, &v.Value, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan service env var: %w", err)
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

func (db *DB) DeleteServiceEnvVar(projectID, serviceID, id int64) error {
	_, err := db.Exec(`DELETE FROM service_env_vars
		WHERE id=? AND service_id IN (
			SELECT s.id FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=? AND s.id=?)`, id, projectID, serviceID)
	if err != nil {
		return fmt.Errorf("delete service env var: %w", err)
	}
	return nil
}

func (db *DB) DeleteServiceEnvVarsByProject(projectID int64) error {
	_, err := db.Exec(`DELETE FROM service_env_vars
		WHERE service_id IN (
			SELECT s.id FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=?)`, projectID)
	if err != nil {
		return fmt.Errorf("delete service env vars by project: %w", err)
	}
	return nil
}
