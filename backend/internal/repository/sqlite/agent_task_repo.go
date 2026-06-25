package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateAgentTask(t *domain.AgentTask) error {
	var sessionID interface{}
	if t.SessionID != nil {
		sessionID = *t.SessionID
	}
	_, err := db.Exec(
		"INSERT INTO agent_tasks (id, project_id, session_id, message, status) VALUES (?, ?, ?, ?, ?)",
		t.ID, t.ProjectID, sessionID, t.Message, t.Status,
	)
	if err != nil {
		return fmt.Errorf("create agent task: %w", err)
	}
	return nil
}

func (db *DB) FindAgentTask(id string) (*domain.AgentTask, error) {
	t := &domain.AgentTask{}
	var sessionID *string
	err := db.QueryRow(
		"SELECT id, project_id, session_id, message, status, COALESCE(response,''), COALESCE(diff,''), created_at, completed_at FROM agent_tasks WHERE id = ?", id,
	).Scan(&t.ID, &t.ProjectID, &sessionID, &t.Message, &t.Status, &t.Response, &t.Diff, &t.CreatedAt, &t.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("find agent task: %w", err)
	}
	t.SessionID = sessionID
	return t, nil
}

func (db *DB) ListAgentTasks(projectID int64) ([]*domain.AgentTask, error) {
	rows, err := db.Query(
		"SELECT id, project_id, session_id, message, status, COALESCE(response,''), COALESCE(diff,''), created_at, completed_at FROM agent_tasks WHERE project_id = ? ORDER BY created_at ASC LIMIT 50",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*domain.AgentTask
	for rows.Next() {
		t := &domain.AgentTask{}
		var sessionID *string
		if err := rows.Scan(&t.ID, &t.ProjectID, &sessionID, &t.Message, &t.Status, &t.Response, &t.Diff, &t.CreatedAt, &t.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan agent task: %w", err)
		}
		t.SessionID = sessionID
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (db *DB) ListAgentTasksBySession(sessionID string) ([]*domain.AgentTask, error) {
	rows, err := db.Query(
		"SELECT id, project_id, session_id, message, status, COALESCE(response,''), COALESCE(diff,''), created_at, completed_at FROM agent_tasks WHERE session_id = ? ORDER BY created_at ASC",
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list agent tasks by session: %w", err)
	}
	defer rows.Close()

	var tasks []*domain.AgentTask
	for rows.Next() {
		t := &domain.AgentTask{}
		var sid *string
		if err := rows.Scan(&t.ID, &t.ProjectID, &sid, &t.Message, &t.Status, &t.Response, &t.Diff, &t.CreatedAt, &t.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan agent task: %w", err)
		}
		t.SessionID = sid
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (db *DB) UpdateAgentTask(t *domain.AgentTask) error {
	_, err := db.Exec(
		"UPDATE agent_tasks SET status=?, response=?, diff=?, completed_at=? WHERE id=?",
		t.Status, t.Response, t.Diff, t.CompletedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent task: %w", err)
	}
	return nil
}
