package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// Detached-session idle timeout - single-row setting, mirrors
// resource_limit_repo.go / egress_settings (see FEAT-022).

// GetIdleTimeoutSettings returns the current global detached-session idle
// timeout. The single row is seeded by migration 000015, so this shouldn't
// return sql.ErrNoRows in practice.
func (db *DB) GetIdleTimeoutSettings() (*domain.IdleTimeoutSettings, error) {
	it := &domain.IdleTimeoutSettings{}
	err := db.QueryRow(`SELECT timeout_seconds FROM idle_timeout_settings WHERE id = 1`).Scan(&it.TimeoutSeconds)
	if err != nil {
		return nil, fmt.Errorf("get idle timeout settings: %w", err)
	}
	return it, nil
}

// UpdateIdleTimeoutSeconds overwrites the single global idle timeout row.
// 0 means Never.
func (db *DB) UpdateIdleTimeoutSeconds(seconds int64) error {
	_, err := db.Exec(`UPDATE idle_timeout_settings SET timeout_seconds = ? WHERE id = 1`, seconds)
	if err != nil {
		return fmt.Errorf("update idle timeout settings: %w", err)
	}
	return nil
}
