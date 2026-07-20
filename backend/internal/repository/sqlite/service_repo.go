package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateService(sealID int64, service *domain.Service) error {
	dependencies, err := json.Marshal(service.Dependencies)
	if err != nil {
		return fmt.Errorf("encode service dependencies: %w", err)
	}
	result, err := db.Exec(`INSERT INTO services (project_id, name, build_context, internal_port, dependencies_json)
		SELECT id, ?, ?, ?, ? FROM projects WHERE id=? AND seal_id=?`,
		service.Name, service.BuildContext, service.InternalPort, string(dependencies), service.ProjectID, sealID)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count created services: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	service.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) FindService(sealID, projectID, serviceID int64) (*domain.Service, error) {
	service := &domain.Service{}
	var dependencies string
	err := db.QueryRow(`SELECT s.id, s.project_id, s.name, s.build_context, s.internal_port,
		s.dependencies_json, s.created_at, s.updated_at
		FROM services s JOIN projects p ON p.id=s.project_id
		WHERE p.seal_id=? AND s.project_id=? AND s.id=?`, sealID, projectID, serviceID).Scan(
		&service.ID, &service.ProjectID, &service.Name, &service.BuildContext, &service.InternalPort,
		&dependencies, &service.CreatedAt, &service.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find service: %w", err)
	}
	if err := json.Unmarshal([]byte(dependencies), &service.Dependencies); err != nil {
		return nil, fmt.Errorf("decode service dependencies: %w", err)
	}
	return service, nil
}

func (db *DB) ListServices(sealID, projectID int64) ([]*domain.Service, error) {
	rows, err := db.Query(`SELECT s.id, s.project_id, s.name, s.build_context, s.internal_port,
		s.dependencies_json, s.created_at, s.updated_at
		FROM services s JOIN projects p ON p.id=s.project_id
		WHERE p.seal_id=? AND s.project_id=? ORDER BY s.id`, sealID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []*domain.Service
	for rows.Next() {
		service := &domain.Service{}
		var dependencies string
		if err := rows.Scan(&service.ID, &service.ProjectID, &service.Name, &service.BuildContext,
			&service.InternalPort, &dependencies, &service.CreatedAt, &service.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		if err := json.Unmarshal([]byte(dependencies), &service.Dependencies); err != nil {
			return nil, fmt.Errorf("decode service dependencies: %w", err)
		}
		services = append(services, service)
	}
	return services, rows.Err()
}
