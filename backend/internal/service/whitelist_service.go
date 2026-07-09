package service

import (
	"fmt"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// WhitelistService owns the agent sandbox egress whitelist: the set of
// domains the sandbox egress proxy permits outbound traffic to (see
// FEAT-006). Defaults are seeded by migration 000010; this just provides
// add/remove/list on top.
type WhitelistService struct {
	db *sqlite.DB
}

func NewWhitelistService(db *sqlite.DB) *WhitelistService {
	return &WhitelistService{db: db}
}

func (s *WhitelistService) List() ([]*domain.WhitelistDomain, error) {
	return s.db.ListWhitelistDomains()
}

// Domains returns just the domain names, normalized, for handing to the
// egress proxy.
func (s *WhitelistService) Domains() ([]string, error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, d := range list {
		out = append(out, d.Domain)
	}
	return out, nil
}

func (s *WhitelistService) Add(domainName string) (*domain.WhitelistDomain, error) {
	normalized := normalizeDomain(domainName)
	if normalized == "" {
		return nil, fmt.Errorf("domain is required")
	}
	d, err := s.db.CreateWhitelistDomain(normalized)
	if err != nil {
		return nil, fmt.Errorf("add domain to whitelist: %w", err)
	}
	return d, nil
}

func (s *WhitelistService) Remove(id int64) error {
	return s.db.DeleteWhitelistDomain(id)
}

func normalizeDomain(d string) string {
	trimmed := strings.TrimSpace(d)
	trimmed = strings.TrimSuffix(trimmed, ".")
	return strings.ToLower(trimmed)
}
