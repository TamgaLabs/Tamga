package sqlite

import (
	"errors"
	"fmt"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

var ErrSealServiceRouteDomainConflict = errors.New("seal service route domain conflict")

func (db *DB) CreateSealServiceRoute(route *domain.SealServiceRoute) error {
	result, err := db.Exec(`INSERT INTO seal_service_routes (seal_id, service_id, domain) VALUES (?, ?, ?)`, route.SealID, route.ServiceID, route.Domain)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: seal_service_routes.domain") {
			return ErrSealServiceRouteDomainConflict
		}
		return fmt.Errorf("create seal service route: %w", err)
	}
	route.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) ListSealServiceRoutes(sealID, serviceID int64) ([]*domain.SealServiceRoute, error) {
	rows, err := db.Query(`SELECT id, seal_id, service_id, domain, created_at FROM seal_service_routes
		WHERE seal_id=? AND service_id=? ORDER BY domain`, sealID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list seal service routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.SealServiceRoute
	for rows.Next() {
		route := &domain.SealServiceRoute{}
		if err := rows.Scan(&route.ID, &route.SealID, &route.ServiceID, &route.Domain, &route.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan seal service route: %w", err)
		}
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (db *DB) DeleteSealServiceRoute(sealID, serviceID, routeID int64) (bool, error) {
	result, err := db.Exec(`DELETE FROM seal_service_routes WHERE id=? AND seal_id=? AND service_id=?`, routeID, sealID, serviceID)
	if err != nil {
		return false, fmt.Errorf("delete seal service route: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count deleted seal service routes: %w", err)
	}
	return affected == 1, nil
}
