package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// ListServiceContainers returns every service/container row belonging to a
// project, ordered by service ID for a stable/deterministic result.
func (db *DB) ListServiceContainers(projectID int64) ([]*domain.ServiceContainer, error) {
	rows, err := db.Query(
		`SELECT sc.id, sc.service_id, sc.container_id, sc.container_name, sc.status, sc.created_at
		 FROM service_containers sc
		 JOIN services s ON s.id=sc.service_id
		 JOIN projects p ON p.id=s.project_id
		 WHERE p.id = ?
		 ORDER BY sc.service_id, sc.id`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list service containers: %w", err)
	}
	defer rows.Close()

	var containers []*domain.ServiceContainer
	for rows.Next() {
		c := &domain.ServiceContainer{}
		if err := rows.Scan(&c.ID, &c.ServiceID, &c.ContainerID, &c.ContainerName, &c.Status, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan service container: %w", err)
		}
		containers = append(containers, c)
	}
	return containers, nil
}

// ReplaceServiceContainers overwrites the full set of service/container rows
// for a project with the given set, atomically (delete-then-insert inside a
// single transaction). This is the deploy engine's primary write path: each
// `up` rewrites the project's whole service-container set rather than
// tracking incremental diffs, which keeps a partial multi-service deploy's
// bookkeeping simple (see TEST-011 §2d/§2e).
func (db *DB) ReplaceServiceContainers(projectID int64, containers []*domain.ServiceContainer) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin replace service containers: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM service_containers
		WHERE service_id IN (SELECT s.id FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=?)`, projectID); err != nil {
		return fmt.Errorf("clear service containers: %w", err)
	}

	for _, c := range containers {
		result, err := tx.Exec(`INSERT INTO service_containers (service_id, container_id, container_name, status)
			SELECT s.id, ?, ?, ? FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=? AND s.id=?`,
			c.ContainerID, c.ContainerName, c.Status, projectID, c.ServiceID,
		)
		if err != nil {
			return fmt.Errorf("insert service container %d: %w", c.ServiceID, err)
		}
		if affected, err := result.RowsAffected(); err != nil {
			return fmt.Errorf("check service container %d ownership: %w", c.ServiceID, err)
		} else if affected != 1 {
			return fmt.Errorf("service %d is not owned by project %d", c.ServiceID, projectID)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace service containers: %w", err)
	}
	return nil
}

// DeleteServiceContainersByProject removes every service/container row for
// a project. service_containers cascades from services (and services from
// projects), so this is only needed when a project's stack is torn down
// without deleting the project itself.
func (db *DB) DeleteServiceContainersByProject(projectID int64) error {
	_, err := db.Exec(`DELETE FROM service_containers
		WHERE service_id IN (SELECT s.id FROM services s JOIN projects p ON p.id=s.project_id WHERE p.id=?)`, projectID)
	if err != nil {
		return fmt.Errorf("delete service containers by project: %w", err)
	}
	return nil
}
