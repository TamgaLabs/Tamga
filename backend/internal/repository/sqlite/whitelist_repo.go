package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) ListWhitelistDomains() ([]*domain.WhitelistDomain, error) {
	rows, err := db.Query(`SELECT id, domain, created_at FROM egress_whitelist ORDER BY domain ASC`)
	if err != nil {
		return nil, fmt.Errorf("list whitelist domains: %w", err)
	}
	defer rows.Close()

	var out []*domain.WhitelistDomain
	for rows.Next() {
		d := &domain.WhitelistDomain{}
		if err := rows.Scan(&d.ID, &d.Domain, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan whitelist domain: %w", err)
		}
		out = append(out, d)
	}
	return out, nil
}

func (db *DB) CreateWhitelistDomain(domainName string) (*domain.WhitelistDomain, error) {
	res, err := db.Exec(`INSERT INTO egress_whitelist (domain) VALUES (?)`, domainName)
	if err != nil {
		return nil, fmt.Errorf("create whitelist domain: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get whitelist domain id: %w", err)
	}
	d := &domain.WhitelistDomain{ID: id, Domain: domainName}
	if err := db.QueryRow(`SELECT created_at FROM egress_whitelist WHERE id = ?`, id).Scan(&d.CreatedAt); err != nil {
		return nil, fmt.Errorf("read back whitelist domain: %w", err)
	}
	return d, nil
}

func (db *DB) DeleteWhitelistDomain(id int64) error {
	_, err := db.Exec(`DELETE FROM egress_whitelist WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete whitelist domain: %w", err)
	}
	return nil
}
