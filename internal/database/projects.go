package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CreateProjectParams struct {
	Name        string
	Description string
	UserID      string
}

type GetProjectByIDParams struct {
	ID     string
	UserID string
}

type UpdateProjectParams struct {
	ID          string
	Name        string
	Description string
	UserID      string
}

type DeleteProjectParams struct {
	ID     string
	UserID string
}

func (q *Queries) CreateProject(ctx context.Context, arg CreateProjectParams) (Project, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, description, user_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, arg.Name, arg.Description, arg.UserID, now, now,
	)
	if err != nil {
		return Project{}, err
	}
	return Project{
		ID: id, Name: arg.Name, Description: arg.Description,
		UserID: arg.UserID, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (q *Queries) ListProjectsByUser(ctx context.Context, userID string) ([]Project, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT id, name, description, user_id, created_at, updated_at FROM projects WHERE user_id = ? ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.UserID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (q *Queries) GetProjectByID(ctx context.Context, arg GetProjectByIDParams) (Project, error) {
	row := q.db.QueryRowContext(ctx,
		`SELECT id, name, description, user_id, created_at, updated_at FROM projects WHERE id = ? AND user_id = ?`,
		arg.ID, arg.UserID,
	)
	var p Project
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.UserID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Project{}, err
	}
	return p, nil
}

func (q *Queries) UpdateProject(ctx context.Context, arg UpdateProjectParams) (Project, error) {
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		arg.Name, arg.Description, now, arg.ID, arg.UserID,
	)
	if err != nil {
		return Project{}, err
	}
	return q.GetProjectByID(ctx, GetProjectByIDParams{ID: arg.ID, UserID: arg.UserID})
}

func (q *Queries) DeleteProject(ctx context.Context, arg DeleteProjectParams) (string, error) {
	_, err := q.db.ExecContext(ctx,
		`DELETE FROM projects WHERE id = ? AND user_id = ?`,
		arg.ID, arg.UserID,
	)
	if err != nil {
		return "", err
	}
	return arg.ID, nil
}
