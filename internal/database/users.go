package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CreateUserParams struct {
	Name         string
	Email        string
	PasswordHash string
}

func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	_, err := q.db.ExecContext(ctx,
		`INSERT INTO users (id, name, email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, arg.Name, arg.Email, arg.PasswordHash, now, now,
	)
	if err != nil {
		return User{}, err
	}
	return User{
		ID: id, Name: arg.Name, Email: arg.Email,
		PasswordHash: arg.PasswordHash, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, created_at, updated_at FROM users WHERE email = ?`, email,
	)
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

func (q *Queries) GetUserByID(ctx context.Context, id string) (User, error) {
	row := q.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, created_at, updated_at FROM users WHERE id = ?`, id,
	)
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}
