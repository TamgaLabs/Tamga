package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateAgentSession(s *domain.AgentSession) error {
	_, err := db.Exec(
		"INSERT INTO agent_sessions (id, project_id, dir_id, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		s.ID, s.ProjectID, s.DirID, s.Name, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create agent session: %w", err)
	}
	return nil
}

func (db *DB) FindAgentSession(id string) (*domain.AgentSession, error) {
	s := &domain.AgentSession{}
	err := db.QueryRow(
		"SELECT id, project_id, dir_id, name, created_at, updated_at FROM agent_sessions WHERE id = ?", id,
	).Scan(&s.ID, &s.ProjectID, &s.DirID, &s.Name, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find agent session: %w", err)
	}
	return s, nil
}

func (db *DB) ListAgentSessions(projectID int64) ([]*domain.AgentSession, error) {
	rows, err := db.Query(
		"SELECT id, project_id, dir_id, name, created_at, updated_at FROM agent_sessions WHERE project_id = ? ORDER BY updated_at DESC LIMIT 50",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.AgentSession
	for rows.Next() {
		s := &domain.AgentSession{}
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.DirID, &s.Name, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan agent session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (db *DB) UpdateAgentSession(s *domain.AgentSession) error {
	_, err := db.Exec(
		"UPDATE agent_sessions SET name=?, updated_at=? WHERE id=?",
		s.Name, s.UpdatedAt, s.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent session: %w", err)
	}
	return nil
}

func (db *DB) DeleteAgentSession(id string) error {
	_, err := db.Exec("DELETE FROM agent_tasks WHERE session_id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent tasks for session: %w", err)
	}
	_, err = db.Exec("DELETE FROM agent_sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent session: %w", err)
	}
	return nil
}
