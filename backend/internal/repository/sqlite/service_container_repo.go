package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// ListServiceContainers returns every service/container row belonging to a
// project, ordered by service name for a stable/deterministic result.
func (db *DB) ListServiceContainers(projectID int64) ([]*domain.ServiceContainer, error) {
	rows, err := db.Query(
		"SELECT id, project_id, service_name, container_id, container_name, status, created_at FROM project_service_containers WHERE project_id = ? ORDER BY service_name",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list service containers: %w", err)
	}
	defer rows.Close()

	var containers []*domain.ServiceContainer
	for rows.Next() {
		c := &domain.ServiceContainer{}
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.ServiceName, &c.ContainerID, &c.ContainerName, &c.Status, &c.CreatedAt); err != nil {
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

	if _, err := tx.Exec("DELETE FROM project_service_containers WHERE project_id = ?", projectID); err != nil {
		return fmt.Errorf("clear service containers: %w", err)
	}

	for _, c := range containers {
		if _, err := tx.Exec(
			"INSERT INTO project_service_containers (project_id, service_name, container_id, container_name, status) VALUES (?, ?, ?, ?, ?)",
			projectID, c.ServiceName, c.ContainerID, c.ContainerName, c.Status,
		); err != nil {
			return fmt.Errorf("insert service container %q: %w", c.ServiceName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace service containers: %w", err)
	}
	return nil
}

// DeleteServiceContainersByProject removes every service/container row for
// a project. project_service_containers already cascades on project
// deletion (migration 000016's ON DELETE CASCADE), so this is only needed
// when a project's stack is torn down without deleting the project itself
// (e.g. a redeploy that fully replaces the set - see ReplaceServiceContainers
// - or a future "stop"/`down` step that isn't part of this task's scope).
func (db *DB) DeleteServiceContainersByProject(projectID int64) error {
	_, err := db.Exec("DELETE FROM project_service_containers WHERE project_id = ?", projectID)
	if err != nil {
		return fmt.Errorf("delete service containers by project: %w", err)
	}
	return nil
}
