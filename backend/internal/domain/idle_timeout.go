package domain

// IdleTimeoutSettings is the single global detached-terminal-session idle
// timeout setting (see FEAT-022). TimeoutSeconds is how long a session may
// sit with no attached WebSocket before AgentService's background sweep
// auto-terminates it, via the same path as an explicit terminate. Single-
// row setting, same pattern as ResourceLimit/EgressSettings.
//
// TimeoutSeconds == 0 means "Never" (the default) - a session persists
// until explicitly terminated, regardless of how long it sits detached.
type IdleTimeoutSettings struct {
	TimeoutSeconds int64 `json:"timeout_seconds"`
}
