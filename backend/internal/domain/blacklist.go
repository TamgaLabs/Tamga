package domain

import "time"

// BlacklistDomain is one entry in the agent sandbox egress blacklist - a
// domain the sandbox egress proxy denies outbound requests to when egress
// mode is "blacklist" (see FEAT-016). Same shape as WhitelistDomain but
// kept as a distinct type since the two lists are semantically different
// and are edited/consumed independently.
type BlacklistDomain struct {
	ID        int64     `json:"id"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}
