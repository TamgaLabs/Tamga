package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateUser(passwordHash string) (*domain.User, error) {
	res, err := db.Exec("INSERT INTO users (password_hash) VALUES (?)", passwordHash)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	id, _ := res.LastInsertId()
	return db.FindUser(id)
}

func (db *DB) FindUser(id int64) (*domain.User, error) {
	u := &domain.User{}
	err := db.QueryRow("SELECT id, password_hash, created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return u, nil
}

func (db *DB) FindFirstUser() (*domain.User, error) {
	u := &domain.User{}
	err := db.QueryRow("SELECT id, password_hash, created_at FROM users ORDER BY id LIMIT 1").
		Scan(&u.ID, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("find first user: %w", err)
	}
	return u, nil
}

func (db *DB) HasUsers() (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count > 0, nil
}
