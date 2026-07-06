package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// GetResourceLimit returns the current global default sandbox resource
// limit. The single row is seeded by migration 000011, so this shouldn't
// return sql.ErrNoRows in practice.
func (db *DB) GetResourceLimit() (*domain.ResourceLimit, error) {
	rl := &domain.ResourceLimit{}
	err := db.QueryRow(`SELECT memory_bytes, nano_cpus FROM resource_limits WHERE id = 1`).
		Scan(&rl.MemoryBytes, &rl.NanoCPUs)
	if err != nil {
		return nil, fmt.Errorf("get resource limit: %w", err)
	}
	return rl, nil
}

// UpdateResourceLimit overwrites the single global default resource limit row.
func (db *DB) UpdateResourceLimit(rl *domain.ResourceLimit) error {
	_, err := db.Exec(`UPDATE resource_limits SET memory_bytes = ?, nano_cpus = ? WHERE id = 1`, rl.MemoryBytes, rl.NanoCPUs)
	if err != nil {
		return fmt.Errorf("update resource limit: %w", err)
	}
	return nil
}
