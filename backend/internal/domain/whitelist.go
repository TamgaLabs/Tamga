package domain

import "time"

// WhitelistDomain is one entry in the agent sandbox egress whitelist - a
// domain the sandbox egress proxy will permit outbound requests to (see
// FEAT-006). Stored as a global setting in SQLite, same pattern as
// AgentProvider/ApiKey.
type WhitelistDomain struct {
	ID        int64     `json:"id"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}
