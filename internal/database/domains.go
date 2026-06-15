package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CreateDomainParams struct {
	ProjectID string
	Domain    string
}

type ListDomainsByProjectParams struct {
	ProjectID string
	UserID    string
}

func (q *Queries) CreateDomain(ctx context.Context, arg CreateDomainParams) (Domain, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO domains (id, project_id, domain, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, arg.ProjectID, arg.Domain, now, now,
	)
	if err != nil {
		return Domain{}, err
	}
	return Domain{
		ID: id, ProjectID: arg.ProjectID, Domain: arg.Domain,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (q *Queries) ListDomainsByProject(ctx context.Context, arg ListDomainsByProjectParams) ([]Domain, error) {
	rows, err := q.db.QueryContext(ctx,
		`SELECT d.id, d.project_id, d.domain, d.verified, d.created_at, d.updated_at FROM domains d
		JOIN projects p ON p.id = d.project_id
		WHERE d.project_id = ? AND p.user_id = ?
		ORDER BY d.created_at DESC`,
		arg.ProjectID, arg.UserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Domain, &d.Verified, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (q *Queries) DeleteDomain(ctx context.Context, id string) (string, error) {
	_, err := q.db.ExecContext(ctx, `DELETE FROM domains WHERE id = ?`, id)
	if err != nil {
		return "", err
	}
	return id, nil
}
