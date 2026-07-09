package service

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// EgressService owns the two new pieces of the agent sandbox egress
// design added by FEAT-016: the global mode setting (open / whitelist /
// blacklist) and the blacklist domain list. WhitelistService (unchanged)
// continues to own the whitelist list itself - see TEST-009 §4 for the
// full design rationale.
type EgressService struct {
	db *sqlite.DB
}

func NewEgressService(db *sqlite.DB) *EgressService {
	return &EgressService{db: db}
}

// GetMode returns the current global egress mode.
func (s *EgressService) GetMode() (domain.EgressMode, error) {
	settings, err := s.db.GetEgressSettings()
	if err != nil {
		return "", err
	}
	return settings.Mode, nil
}

// SetMode overwrites the global egress mode. Must be one of open,
// whitelist or blacklist.
func (s *EgressService) SetMode(mode string) (domain.EgressMode, error) {
	m := domain.EgressMode(mode)
	switch m {
	case domain.EgressModeOpen, domain.EgressModeWhitelist, domain.EgressModeBlacklist:
	default:
		return "", fmt.Errorf("invalid egress mode %q: must be one of open, whitelist, blacklist", mode)
	}
	if err := s.db.UpdateEgressMode(m); err != nil {
		return "", err
	}
	return m, nil
}

func (s *EgressService) ListBlacklist() ([]*domain.BlacklistDomain, error) {
	return s.db.ListBlacklistDomains()
}

// BlacklistDomains returns just the domain names, normalized, for handing
// to the egress proxy.
func (s *EgressService) BlacklistDomains() ([]string, error) {
	list, err := s.ListBlacklist()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, d := range list {
		out = append(out, d.Domain)
	}
	return out, nil
}

func (s *EgressService) AddBlacklist(domainName string) (*domain.BlacklistDomain, error) {
	normalized := normalizeDomain(domainName)
	if normalized == "" {
		return nil, fmt.Errorf("domain is required")
	}
	d, err := s.db.CreateBlacklistDomain(normalized)
	if err != nil {
		return nil, fmt.Errorf("add domain to blacklist: %w", err)
	}
	return d, nil
}

func (s *EgressService) RemoveBlacklist(id int64) error {
	return s.db.DeleteBlacklistDomain(id)
}
