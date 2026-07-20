package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateSeal(seal *domain.Seal) error {
	result, err := db.Exec("INSERT INTO seals (name) VALUES (?)", seal.Name)
	if err != nil {
		return fmt.Errorf("create seal: %w", err)
	}
	seal.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) FindSeal(id int64) (*domain.Seal, error) {
	seal := &domain.Seal{}
	err := db.QueryRow("SELECT id, name, created_at, updated_at FROM seals WHERE id = ?", id).Scan(&seal.ID, &seal.Name, &seal.CreatedAt, &seal.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	return seal, nil
}

func (db *DB) ListSeals() ([]*domain.Seal, error) {
	rows, err := db.Query("SELECT id, name, created_at, updated_at FROM seals ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list seals: %w", err)
	}
	defer rows.Close()

	var seals []*domain.Seal
	for rows.Next() {
		seal := &domain.Seal{}
		if err := rows.Scan(&seal.ID, &seal.Name, &seal.CreatedAt, &seal.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan seal: %w", err)
		}
		seals = append(seals, seal)
	}
	return seals, rows.Err()
}

func (db *DB) UpdateSeal(seal *domain.Seal) error {
	_, err := db.Exec("UPDATE seals SET name=?, updated_at=CURRENT_TIMESTAMP WHERE id=?", seal.Name, seal.ID)
	if err != nil {
		return fmt.Errorf("update seal: %w", err)
	}
	return nil
}

func (db *DB) DeleteSeal(id int64) error {
	if _, err := db.Exec("DELETE FROM seals WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete seal: %w", err)
	}
	return nil
}
