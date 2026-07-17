package sqlite

import (
	"encoding/json"
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateSealService(service *domain.SealService) error {
	dependencies, err := json.Marshal(service.Dependencies)
	if err != nil {
		return fmt.Errorf("encode service dependencies: %w", err)
	}
	result, err := db.Exec(`INSERT INTO seal_services
		(seal_id, repository_id, name, build_context, internal_port, dependencies_json)
		VALUES (?, ?, ?, ?, ?, ?)`, service.SealID, service.RepositoryID, service.Name,
		service.BuildContext, service.InternalPort, string(dependencies))
	if err != nil {
		return fmt.Errorf("create seal service: %w", err)
	}
	service.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) FindSealService(sealID, serviceID int64) (*domain.SealService, error) {
	service := &domain.SealService{}
	var dependencies string
	err := db.QueryRow(`SELECT id, seal_id, repository_id, name, build_context, internal_port,
		dependencies_json, created_at, updated_at FROM seal_services WHERE seal_id=? AND id=?`, sealID, serviceID).Scan(
		&service.ID, &service.SealID, &service.RepositoryID, &service.Name, &service.BuildContext,
		&service.InternalPort, &dependencies, &service.CreatedAt, &service.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find seal service: %w", err)
	}
	if err := json.Unmarshal([]byte(dependencies), &service.Dependencies); err != nil {
		return nil, fmt.Errorf("decode seal service dependencies: %w", err)
	}
	return service, nil
}

func (db *DB) ListSealServices(sealID int64) ([]*domain.SealService, error) {
	rows, err := db.Query(`SELECT id, seal_id, repository_id, name, build_context, internal_port,
		dependencies_json, created_at, updated_at FROM seal_services WHERE seal_id=? ORDER BY id`, sealID)
	if err != nil {
		return nil, fmt.Errorf("list seal services: %w", err)
	}
	defer rows.Close()
	var services []*domain.SealService
	for rows.Next() {
		service := &domain.SealService{}
		var dependencies string
		if err := rows.Scan(&service.ID, &service.SealID, &service.RepositoryID, &service.Name,
			&service.BuildContext, &service.InternalPort, &dependencies, &service.CreatedAt, &service.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan seal service: %w", err)
		}
		if err := json.Unmarshal([]byte(dependencies), &service.Dependencies); err != nil {
			return nil, fmt.Errorf("decode seal service dependencies: %w", err)
		}
		services = append(services, service)
	}
	return services, rows.Err()
}
