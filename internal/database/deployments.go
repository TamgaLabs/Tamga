package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type GetDeploymentByIDParams struct {
	ID     string
	UserID string
}

type ListDeploymentsByProjectParams struct {
	ProjectID string
	UserID    string
}

type GetLatestDeploymentByProjectParams struct {
	ProjectID string
	UserID    string
}

type UpdateDeploymentDetailsParams struct {
	ID            string
	Status        string
	CommitSha     string
	CommitMessage string
	ImageTag      string
	ContainerID   string
	Domain        string
}

type UpdateDeploymentStatusParams struct {
	ID     string
	Status string
}

type CreateDeploymentLogParams struct {
	DeploymentID string
	Stream       string
	Message      string
}

func (q *Queries) CreateDeployment(ctx context.Context, projectID string) (Deployment, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO deployments (id, project_id, status, created_at, updated_at) VALUES (?, ?, 'pending', ?, ?)`,
		id, projectID, now, now,
	)
	if err != nil {
		return Deployment{}, err
	}
	return Deployment{
		ID: id, ProjectID: projectID, Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (q *Queries) CreateDeploymentLog(ctx context.Context, arg CreateDeploymentLogParams) (DeploymentLog, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO deployment_logs (id, deployment_id, stream, message, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, arg.DeploymentID, arg.Stream, arg.Message, now,
	)
	if err != nil {
		return DeploymentLog{}, err
	}
	return DeploymentLog{
		ID: id, DeploymentID: arg.DeploymentID,
		Stream: arg.Stream, Message: arg.Message, CreatedAt: now,
	}, nil
}

func (q *Queries) GetDeploymentByID(ctx context.Context, arg GetDeploymentByIDParams) (Deployment, error) {
	row := q.db.QueryRowContext(ctx,
		`SELECT d.id, d.project_id, d.status, d.commit_sha, d.commit_message, d.image_tag, d.container_id, d.domain, d.created_at, d.updated_at FROM deployments d
		JOIN projects p ON p.id = d.project_id
		WHERE d.id = ? AND p.user_id = ?`,
		arg.ID, arg.UserID,
	)
	var d Deployment
	err := row.Scan(&d.ID, &d.ProjectID, &d.Status, &d.CommitSha, &d.CommitMessage,
		&d.ImageTag, &d.ContainerID, &d.Domain, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return Deployment{}, err
	}
	return d, nil
}

func (q *Queries) GetDeploymentByIDNoAuth(ctx context.Context, id string) (Deployment, error) {
	row := q.db.QueryRowContext(ctx,
		`SELECT id, project_id, status, commit_sha, commit_message, image_tag, container_id, domain, created_at, updated_at FROM deployments WHERE id = ?`,
		id,
	)
	var d Deployment
	err := row.Scan(&d.ID, &d.ProjectID, &d.Status, &d.CommitSha, &d.CommitMessage,
		&d.ImageTag, &d.ContainerID, &d.Domain, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return Deployment{}, err
	}
	return d, nil
}

func (q *Queries) GetLatestDeploymentByProject(ctx context.Context, arg GetLatestDeploymentByProjectParams) (Deployment, error) {
	row := q.db.QueryRowContext(ctx,
		`SELECT d.id, d.project_id, d.status, d.commit_sha, d.commit_message, d.image_tag, d.container_id, d.domain, d.created_at, d.updated_at FROM deployments d
		JOIN projects p ON p.id = d.project_id
		WHERE d.project_id = ? AND p.user_id = ?
		ORDER BY d.created_at DESC LIMIT 1`,
		arg.ProjectID, arg.UserID,
	)
	var d Deployment
	err := row.Scan(&d.ID, &d.ProjectID, &d.Status, &d.CommitSha, &d.CommitMessage,
		&d.ImageTag, &d.ContainerID, &d.Domain, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return Deployment{}, err
	}
	return d, nil
}

func (q *Queries) ListDeploymentLogs(ctx context.Context, deploymentID string) ([]DeploymentLog, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT id, deployment_id, stream, message, created_at FROM deployment_logs WHERE deployment_id = ? ORDER BY created_at ASC`,
		deploymentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []DeploymentLog
	for rows.Next() {
		var l DeploymentLog
		if err := rows.Scan(&l.ID, &l.DeploymentID, &l.Stream, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

func (q *Queries) ListDeploymentsByProject(ctx context.Context, arg ListDeploymentsByProjectParams) ([]Deployment, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT d.id, d.project_id, d.status, d.commit_sha, d.commit_message, d.image_tag, d.container_id, d.domain, d.created_at, d.updated_at FROM deployments d
		JOIN projects p ON p.id = d.project_id
		WHERE d.project_id = ? AND p.user_id = ?
		ORDER BY d.created_at DESC`,
		arg.ProjectID, arg.UserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Status, &d.CommitSha, &d.CommitMessage,
			&d.ImageTag, &d.ContainerID, &d.Domain, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (q *Queries) UpdateDeploymentDetails(ctx context.Context, arg UpdateDeploymentDetailsParams) (Deployment, error) {
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`UPDATE deployments SET status = ?, commit_sha = ?, commit_message = ?, image_tag = ?, container_id = ?, domain = ?, updated_at = ? WHERE id = ?`,
		arg.Status, arg.CommitSha, arg.CommitMessage, arg.ImageTag, arg.ContainerID, arg.Domain, now, arg.ID,
	)
	if err != nil {
		return Deployment{}, err
	}
	return q.GetDeploymentByIDNoAuth(ctx, arg.ID)
}

func (q *Queries) UpdateDeploymentStatus(ctx context.Context, arg UpdateDeploymentStatusParams) (Deployment, error) {
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`UPDATE deployments SET status = ?, updated_at = ? WHERE id = ?`,
		arg.Status, now, arg.ID,
	)
	if err != nil {
		return Deployment{}, err
	}
	return q.GetDeploymentByIDNoAuth(ctx, arg.ID)
}
