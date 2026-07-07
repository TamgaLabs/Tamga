package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateAgentProvider(p *domain.AgentProvider) error {
	if !p.IsDefault {
		_, err := db.Exec(
			`INSERT INTO agent_providers (id, name, type, image, env, is_default)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			p.ID, p.Name, p.Type, p.Image, p.Env, p.IsDefault,
		)
		if err != nil {
			return fmt.Errorf("create agent provider: %w", err)
		}
		return nil
	}

	// Setting this provider as default must atomically clear is_default on
	// every other row so at most one row is ever the default at a time.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("create agent provider: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE agent_providers SET is_default = 0`); err != nil {
		return fmt.Errorf("create agent provider: clear existing default: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO agent_providers (id, name, type, image, env, is_default)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Type, p.Image, p.Env, p.IsDefault,
	); err != nil {
		return fmt.Errorf("create agent provider: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create agent provider: %w", err)
	}
	return nil
}

func (db *DB) FindAgentProvider(id string) (*domain.AgentProvider, error) {
	p := &domain.AgentProvider{}
	err := db.QueryRow(
		`SELECT id, name, type, COALESCE(image,''), COALESCE(env,'{}'), is_default, created_at, updated_at
		 FROM agent_providers WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Type, &p.Image, &p.Env, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find agent provider: %w", err)
	}
	return p, nil
}

func (db *DB) FindDefaultProvider() (*domain.AgentProvider, error) {
	p := &domain.AgentProvider{}
	// ORDER BY is a defensive second line of defense for deterministic
	// behavior; the real invariant (at most one is_default=1 row) is
	// enforced in CreateAgentProvider/UpdateAgentProvider.
	err := db.QueryRow(
		`SELECT id, name, type, COALESCE(image,''), COALESCE(env,'{}'), is_default, created_at, updated_at
		 FROM agent_providers WHERE is_default = 1 ORDER BY id ASC LIMIT 1`,
	).Scan(&p.ID, &p.Name, &p.Type, &p.Image, &p.Env, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find default provider: %w", err)
	}
	return p, nil
}

func (db *DB) ListAgentProviders() ([]*domain.AgentProvider, error) {
	rows, err := db.Query(
		`SELECT id, name, type, COALESCE(image,''), COALESCE(env,'{}'), is_default, created_at, updated_at
		 FROM agent_providers ORDER BY is_default DESC, name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent providers: %w", err)
	}
	defer rows.Close()

	var providers []*domain.AgentProvider
	for rows.Next() {
		p := &domain.AgentProvider{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Image, &p.Env, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent provider: %w", err)
		}
		providers = append(providers, p)
	}
	return providers, nil
}

func (db *DB) UpdateAgentProvider(p *domain.AgentProvider) error {
	if !p.IsDefault {
		_, err := db.Exec(
			`UPDATE agent_providers SET name=?, type=?, image=?, env=?, is_default=?, updated_at=CURRENT_TIMESTAMP
			 WHERE id=?`,
			p.Name, p.Type, p.Image, p.Env, p.IsDefault, p.ID,
		)
		if err != nil {
			return fmt.Errorf("update agent provider: %w", err)
		}
		return nil
	}

	// Setting this provider as default must atomically clear is_default on
	// every other row so at most one row is ever the default at a time.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("update agent provider: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE agent_providers SET is_default = 0 WHERE id != ?`, p.ID); err != nil {
		return fmt.Errorf("update agent provider: clear existing default: %w", err)
	}

	if _, err := tx.Exec(
		`UPDATE agent_providers SET name=?, type=?, image=?, env=?, is_default=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		p.Name, p.Type, p.Image, p.Env, p.IsDefault, p.ID,
	); err != nil {
		return fmt.Errorf("update agent provider: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("update agent provider: %w", err)
	}
	return nil
}

func (db *DB) DeleteAgentProvider(id string) error {
	_, err := db.Exec("DELETE FROM agent_providers WHERE id = ? AND is_default = 0", id)
	if err != nil {
		return fmt.Errorf("delete agent provider: %w", err)
	}
	return nil
}
