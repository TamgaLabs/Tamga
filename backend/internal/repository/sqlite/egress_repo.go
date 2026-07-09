package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// Blacklist domain CRUD - mirrors whitelist_repo.go's whitelist functions.

func (db *DB) ListBlacklistDomains() ([]*domain.BlacklistDomain, error) {
	rows, err := db.Query(`SELECT id, domain, created_at FROM egress_blacklist ORDER BY domain ASC`)
	if err != nil {
		return nil, fmt.Errorf("list blacklist domains: %w", err)
	}
	defer rows.Close()

	var out []*domain.BlacklistDomain
	for rows.Next() {
		d := &domain.BlacklistDomain{}
		if err := rows.Scan(&d.ID, &d.Domain, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan blacklist domain: %w", err)
		}
		out = append(out, d)
	}
	return out, nil
}

func (db *DB) CreateBlacklistDomain(domainName string) (*domain.BlacklistDomain, error) {
	res, err := db.Exec(`INSERT INTO egress_blacklist (domain) VALUES (?)`, domainName)
	if err != nil {
		return nil, fmt.Errorf("create blacklist domain: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get blacklist domain id: %w", err)
	}
	d := &domain.BlacklistDomain{ID: id, Domain: domainName}
	if err := db.QueryRow(`SELECT created_at FROM egress_blacklist WHERE id = ?`, id).Scan(&d.CreatedAt); err != nil {
		return nil, fmt.Errorf("read back blacklist domain: %w", err)
	}
	return d, nil
}

func (db *DB) DeleteBlacklistDomain(id int64) error {
	_, err := db.Exec(`DELETE FROM egress_blacklist WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete blacklist domain: %w", err)
	}
	return nil
}

// Egress mode - single-row setting, mirrors resource_limit_repo.go.

// GetEgressSettings returns the current global egress mode. The single row
// is seeded by migration 000014, so this shouldn't return sql.ErrNoRows in
// practice.
func (db *DB) GetEgressSettings() (*domain.EgressSettings, error) {
	es := &domain.EgressSettings{}
	err := db.QueryRow(`SELECT mode FROM egress_settings WHERE id = 1`).Scan(&es.Mode)
	if err != nil {
		return nil, fmt.Errorf("get egress settings: %w", err)
	}
	return es, nil
}

// UpdateEgressMode overwrites the single global egress mode row.
func (db *DB) UpdateEgressMode(mode domain.EgressMode) error {
	_, err := db.Exec(`UPDATE egress_settings SET mode = ? WHERE id = 1`, mode)
	if err != nil {
		return fmt.Errorf("update egress mode: %w", err)
	}
	return nil
}
