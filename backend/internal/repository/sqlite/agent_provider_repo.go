package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateAgentProvider(p *domain.AgentProvider) error {
	_, err := db.Exec(
		`INSERT INTO agent_providers (id, name, type, image, command, endpoint, auth_token, env, is_default)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Type, p.Image, p.Command, p.Endpoint, p.AuthToken, p.Env, p.IsDefault,
	)
	if err != nil {
		return fmt.Errorf("create agent provider: %w", err)
	}
	return nil
}

func (db *DB) FindAgentProvider(id string) (*domain.AgentProvider, error) {
	p := &domain.AgentProvider{}
	err := db.QueryRow(
		`SELECT id, name, type, COALESCE(image,''), COALESCE(command,''), COALESCE(endpoint,''),
		        COALESCE(auth_token,''), COALESCE(env,'{}'), is_default, created_at, updated_at
		 FROM agent_providers WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Type, &p.Image, &p.Command, &p.Endpoint, &p.AuthToken, &p.Env, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find agent provider: %w", err)
	}
	return p, nil
}

func (db *DB) FindDefaultProvider() (*domain.AgentProvider, error) {
	p := &domain.AgentProvider{}
	err := db.QueryRow(
		`SELECT id, name, type, COALESCE(image,''), COALESCE(command,''), COALESCE(endpoint,''),
		        COALESCE(auth_token,''), COALESCE(env,'{}'), is_default, created_at, updated_at
		 FROM agent_providers WHERE is_default = 1 LIMIT 1`,
	).Scan(&p.ID, &p.Name, &p.Type, &p.Image, &p.Command, &p.Endpoint, &p.AuthToken, &p.Env, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find default provider: %w", err)
	}
	return p, nil
}

func (db *DB) ListAgentProviders() ([]*domain.AgentProvider, error) {
	rows, err := db.Query(
		`SELECT id, name, type, COALESCE(image,''), COALESCE(command,''), COALESCE(endpoint,''),
		        COALESCE(auth_token,''), COALESCE(env,'{}'), is_default, created_at, updated_at
		 FROM agent_providers ORDER BY is_default DESC, name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent providers: %w", err)
	}
	defer rows.Close()

	var providers []*domain.AgentProvider
	for rows.Next() {
		p := &domain.AgentProvider{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Image, &p.Command, &p.Endpoint, &p.AuthToken, &p.Env, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent provider: %w", err)
		}
		providers = append(providers, p)
	}
	return providers, nil
}

func (db *DB) UpdateAgentProvider(p *domain.AgentProvider) error {
	_, err := db.Exec(
		`UPDATE agent_providers SET name=?, type=?, image=?, command=?, endpoint=?, auth_token=?, env=?, is_default=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		p.Name, p.Type, p.Image, p.Command, p.Endpoint, p.AuthToken, p.Env, p.IsDefault, p.ID,
	)
	if err != nil {
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
