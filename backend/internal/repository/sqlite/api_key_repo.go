package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateApiKey(k *domain.ApiKey) error {
	_, err := db.Exec(
		`INSERT INTO api_keys (id, provider, key_enc, label)
		 VALUES (?, ?, ?, ?)`,
		k.ID, k.Provider, k.KeyEnc, k.Label,
	)
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	// Read back the created row to populate timestamps from database defaults
	return db.populateTimestamps(k)
}

func (db *DB) populateTimestamps(k *domain.ApiKey) error {
	return db.QueryRow(
		`SELECT created_at, updated_at FROM api_keys WHERE id = ?`,
		k.ID,
	).Scan(&k.CreatedAt, &k.UpdatedAt)
}

func (db *DB) ListApiKeys() ([]*domain.ApiKey, error) {
	rows, err := db.Query(
		`SELECT id, provider, key_enc, COALESCE(label,''), created_at, updated_at
		 FROM api_keys ORDER BY provider ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*domain.ApiKey
	for rows.Next() {
		k := &domain.ApiKey{}
		if err := rows.Scan(&k.ID, &k.Provider, &k.KeyEnc, &k.Label, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (db *DB) FindApiKey(id string) (*domain.ApiKey, error) {
	k := &domain.ApiKey{}
	err := db.QueryRow(
		`SELECT id, provider, key_enc, COALESCE(label,''), created_at, updated_at
		 FROM api_keys WHERE id = ?`, id,
	).Scan(&k.ID, &k.Provider, &k.KeyEnc, &k.Label, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find api key: %w", err)
	}
	return k, nil
}

func (db *DB) FindApiKeyByProvider(provider string) (*domain.ApiKey, error) {
	k := &domain.ApiKey{}
	err := db.QueryRow(
		`SELECT id, provider, key_enc, COALESCE(label,''), created_at, updated_at
		 FROM api_keys WHERE provider = ? LIMIT 1`, provider,
	).Scan(&k.ID, &k.Provider, &k.KeyEnc, &k.Label, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find api key by provider: %w", err)
	}
	return k, nil
}

func (db *DB) DeleteApiKey(id string) error {
	_, err := db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}
