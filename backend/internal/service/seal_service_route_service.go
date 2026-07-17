package service

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

var (
	ErrInvalidSealServiceRoute  = errors.New("route must be an exact domain")
	ErrSealServiceRouteConflict = errors.New("route domain already exists")
	ErrSealServiceRouteNotFound = errors.New("route not found")
)

type CreateSealServiceRouteRequest struct {
	Domain string `json:"domain"`
}

// AddServiceRoute assigns one normalized exact domain to a Seal service. The
// database unique key is the conflict authority, so a failed insert cannot
// alter other persisted routes.
func (s *SealService) AddServiceRoute(ctx context.Context, sealID, serviceID int64, req CreateSealServiceRouteRequest) (*domain.SealServiceRoute, error) {
	if _, err := s.db.FindSealService(sealID, serviceID); err != nil {
		return nil, fmt.Errorf("find seal service: %w", err)
	}
	domainName := normalizeExactDomain(req.Domain)
	if !validExactDomain(domainName) {
		return nil, ErrInvalidSealServiceRoute
	}
	route := &domain.SealServiceRoute{SealID: sealID, ServiceID: serviceID, Domain: domainName}
	if err := s.db.CreateSealServiceRoute(route); err != nil {
		if errors.Is(err, sqlite.ErrSealServiceRouteDomainConflict) {
			return nil, ErrSealServiceRouteConflict
		}
		return nil, err
	}
	return route, nil
}

func (s *SealService) ListServiceRoutes(ctx context.Context, sealID, serviceID int64) ([]*domain.SealServiceRoute, error) {
	if _, err := s.db.FindSealService(sealID, serviceID); err != nil {
		return nil, fmt.Errorf("find seal service: %w", err)
	}
	return s.db.ListSealServiceRoutes(sealID, serviceID)
}

func (s *SealService) DeleteServiceRoute(ctx context.Context, sealID, serviceID, routeID int64) error {
	if _, err := s.db.FindSealService(sealID, serviceID); err != nil {
		return fmt.Errorf("find seal service: %w", err)
	}
	deleted, err := s.db.DeleteSealServiceRoute(sealID, serviceID, routeID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrSealServiceRouteNotFound
	}
	return nil
}

func normalizeExactDomain(domainName string) string {
	return strings.ToLower(strings.TrimSpace(domainName))
}

func validExactDomain(domainName string) bool {
	if len(domainName) == 0 || len(domainName) > 253 || strings.ContainsAny(domainName, "/:@`*? ") {
		return false
	}
	if _, err := netip.ParseAddr(domainName); err == nil {
		return false
	}
	labels := strings.Split(domainName, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}
