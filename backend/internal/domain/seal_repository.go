package domain

import "time"

// SealRepository is a remote Git checkout owned by one Seal. WorkspacePath is
// always relative to that Seal's workspace and is assigned by SealService.
type SealRepository struct {
	ID            int64               `json:"id"`
	SealID        int64               `json:"seal_id"`
	DisplayName   string              `json:"display_name"`
	RemoteURL     string              `json:"remote_url"`
	Branch        string              `json:"branch"`
	WorkspacePath string              `json:"workspace_path"`
	Status        ProjectSourceStatus `json:"status"`
	ErrorSummary  string              `json:"error_summary,omitempty"`
	CreatedAt     time.Time           `json:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at"`
}
