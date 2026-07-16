package sqlite

import (
	"fmt"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) ListProjectRoutes(projectID int64) ([]*domain.ProjectRoute, error) {
	rows, err := db.Query("SELECT id, seal_id, service_name, domain FROM seal_routes WHERE seal_id=? ORDER BY domain", projectID)
	if err != nil {
		return nil, fmt.Errorf("list project routes: %w", err)
	}
	defer rows.Close()
	var routes []*domain.ProjectRoute
	for rows.Next() {
		r := &domain.ProjectRoute{}
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Service, &r.Domain); err != nil {
			return nil, err
		}
		routes = append(routes, r)
	}
	return routes, rows.Err()
}

func (db *DB) ReplaceProjectRoutes(projectID int64, routes []*domain.ProjectRoute) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM seal_routes WHERE seal_id=?", projectID); err != nil {
		return err
	}
	for _, route := range routes {
		route.ProjectID, route.Domain = projectID, strings.ToLower(strings.TrimSpace(route.Domain))
		result, err := tx.Exec("INSERT INTO seal_routes (seal_id, service_name, domain) VALUES (?, ?, ?)", projectID, route.Service, route.Domain)
		if err != nil {
			return fmt.Errorf("save project route: %w", err)
		}
		route.ID, _ = result.LastInsertId()
	}
	return tx.Commit()
}
