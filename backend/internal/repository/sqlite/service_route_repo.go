package sqlite

import (
	"fmt"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

var ErrServiceRouteDomainConflict = domain.ErrAlreadyExists

func (db *DB) CreateServiceRoute(sealID, projectID int64, route *domain.ServiceRoute) error {
	route.Domain = strings.ToLower(strings.TrimSpace(route.Domain))
	result, err := db.Exec(`INSERT INTO service_routes (service_id, domain)
		SELECT s.id, ? FROM services s JOIN projects p ON p.id=s.project_id
		WHERE p.seal_id=? AND s.project_id=? AND s.id=?`, route.Domain, sealID, projectID, route.ServiceID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: service_routes.domain") {
			return ErrServiceRouteDomainConflict
		}
		return fmt.Errorf("create service route: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count created service routes: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	route.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) ListServiceRoutes(sealID, projectID, serviceID int64) ([]*domain.ServiceRoute, error) {
	rows, err := db.Query(`SELECT r.id, r.service_id, r.domain, r.created_at
		FROM service_routes r JOIN services s ON s.id=r.service_id
		JOIN projects p ON p.id=s.project_id
		WHERE p.seal_id=? AND s.project_id=? AND s.id=? ORDER BY r.domain`, sealID, projectID, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list service routes: %w", err)
	}
	defer rows.Close()

	var routes []*domain.ServiceRoute
	for rows.Next() {
		route := &domain.ServiceRoute{}
		if err := rows.Scan(&route.ID, &route.ServiceID, &route.Domain, &route.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan service route: %w", err)
		}
		routes = append(routes, route)
	}
	return routes, rows.Err()
}

func (db *DB) DeleteServiceRoute(sealID, projectID, serviceID, routeID int64) (bool, error) {
	result, err := db.Exec(`DELETE FROM service_routes WHERE id=? AND service_id=?
		AND EXISTS (
			SELECT 1 FROM services s JOIN projects p ON p.id=s.project_id
			WHERE p.seal_id=? AND s.project_id=? AND s.id=?
		)`, routeID, serviceID, sealID, projectID, serviceID)
	if err != nil {
		return false, fmt.Errorf("delete service route: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count deleted service routes: %w", err)
	}
	return affected == 1, nil
}
