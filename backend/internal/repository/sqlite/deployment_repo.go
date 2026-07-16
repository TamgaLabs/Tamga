package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateDeployment(d *domain.Deployment) error {
	res, err := db.Exec(
		"INSERT INTO deployments (seal_id, status, commit_sha, logs) VALUES (?, ?, ?, ?)",
		d.ProjectID, d.Status, d.CommitSHA, d.Logs,
	)
	if err != nil {
		return fmt.Errorf("create deployment: %w", err)
	}
	id, _ := res.LastInsertId()
	d.ID = id
	return nil
}

func (db *DB) FindDeployment(id int64) (*domain.Deployment, error) {
	d := &domain.Deployment{}
	err := db.QueryRow(
		"SELECT id, seal_id, status, commit_sha, logs, created_at, updated_at FROM deployments WHERE id = ?", id,
	).Scan(&d.ID, &d.ProjectID, &d.Status, &d.CommitSHA, &d.Logs, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find deployment: %w", err)
	}
	return d, nil
}

func (db *DB) ListDeployments(projectID int64) ([]*domain.Deployment, error) {
	rows, err := db.Query(
		"SELECT id, seal_id, status, commit_sha, logs, created_at, updated_at FROM deployments WHERE seal_id = ? ORDER BY created_at DESC",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	defer rows.Close()

	var deployments []*domain.Deployment
	for rows.Next() {
		d := &domain.Deployment{}
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Status, &d.CommitSHA, &d.Logs, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan deployment: %w", err)
		}
		deployments = append(deployments, d)
	}
	return deployments, nil
}

func (db *DB) UpdateDeployment(d *domain.Deployment) error {
	_, err := db.Exec(
		"UPDATE deployments SET status=?, commit_sha=?, logs=?, updated_at=CURRENT_TIMESTAMP WHERE id=?",
		d.Status, d.CommitSHA, d.Logs, d.ID,
	)
	if err != nil {
		return fmt.Errorf("update deployment: %w", err)
	}
	return nil
}

func (db *DB) DeleteDeploymentsByProject(projectID int64) error {
	_, err := db.Exec("DELETE FROM deployments WHERE seal_id = ?", projectID)
	if err != nil {
		return fmt.Errorf("delete deployments: %w", err)
	}
	return nil
}
