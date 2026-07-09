package domain

// EgressMode is the sandbox egress proxy's traffic policy (see FEAT-016):
//   - "open": all outbound traffic allowed (the default).
//   - "whitelist": only domains on the whitelist are allowed.
//   - "blacklist": all outbound traffic allowed except domains on the
//     blacklist.
type EgressMode string

const (
	EgressModeOpen      EgressMode = "open"
	EgressModeWhitelist EgressMode = "whitelist"
	EgressModeBlacklist EgressMode = "blacklist"
)

// EgressSettings is the single global sandbox egress mode setting.
// Single-row setting, same pattern as ResourceLimit.
type EgressSettings struct {
	Mode EgressMode `json:"mode"`
}
