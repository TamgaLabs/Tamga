package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CreateGitRepositoryParams struct {
	ProjectID string
	Url       string
	Branch    string
}

type GetGitRepositoryByProjectParams struct {
	ProjectID string
	UserID    string
}

func (q *Queries) CreateGitRepository(ctx context.Context, arg CreateGitRepositoryParams) (GitRepository, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO git_repositories (id, project_id, url, branch, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, arg.ProjectID, arg.Url, arg.Branch, now, now,
	)
	if err != nil {
		return GitRepository{}, err
	}
	return GitRepository{
		ID: id, ProjectID: arg.ProjectID, Url: arg.Url, Branch: arg.Branch,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (q *Queries) GetGitRepositoryByProject(ctx context.Context, arg GetGitRepositoryByProjectParams) (GitRepository, error) {
	row := q.db.QueryRowContext(ctx,
		`SELECT g.id, g.project_id, g.url, g.branch, g.created_at, g.updated_at FROM git_repositories g
		JOIN projects p ON p.id = g.project_id
		WHERE g.project_id = ? AND p.user_id = ?`,
		arg.ProjectID, arg.UserID,
	)
	var r GitRepository
	err := row.Scan(&r.ID, &r.ProjectID, &r.Url, &r.Branch, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return GitRepository{}, err
	}
	return r, nil
}

func (q *Queries) DeleteGitRepository(ctx context.Context, id string) (string, error) {
	_, err := q.db.ExecContext(ctx, `DELETE FROM git_repositories WHERE id = ?`, id)
	if err != nil {
		return "", err
	}
	return id, nil
}
