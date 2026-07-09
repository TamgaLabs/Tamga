package domain

import "time"

// GitCredential is the single, global git credential used both by the
// backend for `git clone`/`pull` (project_service.go) and injected into
// every agent sandbox so a user can `git commit`/`push` from the terminal
// (agent_service.go). Per architecture.md there is exactly one - not a
// per-project or per-provider list - so this mirrors the single-row shape
// of ResourceLimit rather than the list shape of WhitelistDomain.
type GitCredential struct {
	Provider  string    `json:"provider"`
	Username  string    `json:"username"`
	TokenEnc  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type GitCredentialResponse struct {
	Provider  string    `json:"provider"`
	Username  string    `json:"username"`
	HasToken  bool      `json:"has_token"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
