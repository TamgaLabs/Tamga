package service

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// IdleTimeoutService owns the single global detached-terminal-session idle
// timeout setting (see FEAT-022): how long a session may sit with no
// attached WebSocket before AgentService's background sweep (see
// agent_service.go's sweepIdleSessions) auto-terminates it. Same single-row
// Get/Set pattern as ResourceLimitService/EgressService - there's exactly
// one setting here, not a list to CRUD.
type IdleTimeoutService struct {
	db *sqlite.DB
}

func NewIdleTimeoutService(db *sqlite.DB) *IdleTimeoutService {
	return &IdleTimeoutService{db: db}
}

func (s *IdleTimeoutService) Get() (*domain.IdleTimeoutSettings, error) {
	return s.db.GetIdleTimeoutSettings()
}

// Set overwrites the global idle timeout, in seconds. 0 means Never
// (sessions persist until explicitly terminated) - negative values are
// rejected. Takes effect on the next sweep tick, no restart required.
func (s *IdleTimeoutService) Set(seconds int64) (*domain.IdleTimeoutSettings, error) {
	if seconds < 0 {
		return nil, fmt.Errorf("timeout_seconds must be >= 0 (0 means never)")
	}
	if err := s.db.UpdateIdleTimeoutSeconds(seconds); err != nil {
		return nil, err
	}
	return &domain.IdleTimeoutSettings{TimeoutSeconds: seconds}, nil
}
