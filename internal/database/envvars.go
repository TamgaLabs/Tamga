package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CreateEnvVarParams struct {
	ProjectID string
	Key       string
	Value     string
}

type ListEnvVarsByProjectParams struct {
	ProjectID string
	UserID    string
}

type UpdateEnvVarParams struct {
	ID    string
	Key   string
	Value string
}

func (q *Queries) CreateEnvVar(ctx context.Context, arg CreateEnvVarParams) (EnvVar, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO env_vars (id, project_id, key, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, arg.ProjectID, arg.Key, arg.Value, now, now,
	)
	if err != nil {
		return EnvVar{}, err
	}
	return EnvVar{
		ID: id, ProjectID: arg.ProjectID, Key: arg.Key, Value: arg.Value,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (q *Queries) ListEnvVarsByProject(ctx context.Context, arg ListEnvVarsByProjectParams) ([]EnvVar, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT e.id, e.project_id, e.key, e.value, e.created_at, e.updated_at FROM env_vars e
		JOIN projects p ON p.id = e.project_id
		WHERE e.project_id = ? AND p.user_id = ?
		ORDER BY e.key ASC`,
		arg.ProjectID, arg.UserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []EnvVar
	for rows.Next() {
		var e EnvVar
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Key, &e.Value, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func (q *Queries) UpdateEnvVar(ctx context.Context, arg UpdateEnvVarParams) (EnvVar, error) {
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`UPDATE env_vars SET key = ?, value = ?, updated_at = ? WHERE id = ?`,
		arg.Key, arg.Value, now, arg.ID,
	)
	if err != nil {
		return EnvVar{}, err
	}
	row := q.db.QueryRowContext(ctx,
		`SELECT id, project_id, key, value, created_at, updated_at FROM env_vars WHERE id = ?`, arg.ID,
	)
	var e EnvVar
	err = row.Scan(&e.ID, &e.ProjectID, &e.Key, &e.Value, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return EnvVar{}, err
	}
	return e, nil
}

func (q *Queries) DeleteEnvVar(ctx context.Context, id string) (string, error) {
	_, err := q.db.ExecContext(ctx, `DELETE FROM env_vars WHERE id = ?`, id)
	if err != nil {
		return "", err
	}
	return id, nil
}
