package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// GetGitCredential returns the current global git credential. The single
// row is seeded by migration 000012, so this shouldn't return
// sql.ErrNoRows in practice. TokenEnc is empty when no credential has been
// configured yet.
func (db *DB) GetGitCredential() (*domain.GitCredential, error) {
	c := &domain.GitCredential{}
	err := db.QueryRow(
		`SELECT provider, username, token_enc, created_at, updated_at FROM git_credential WHERE id = 1`,
	).Scan(&c.Provider, &c.Username, &c.TokenEnc, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get git credential: %w", err)
	}
	return c, nil
}

// UpdateGitCredential overwrites the single global git credential row.
func (db *DB) UpdateGitCredential(c *domain.GitCredential) error {
	_, err := db.Exec(
		`UPDATE git_credential SET provider = ?, username = ?, token_enc = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`,
		c.Provider, c.Username, c.TokenEnc,
	)
	if err != nil {
		return fmt.Errorf("update git credential: %w", err)
	}
	return nil
}
