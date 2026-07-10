CREATE TABLE IF NOT EXISTS idle_timeout_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    timeout_seconds INTEGER NOT NULL DEFAULT 0
);

-- Single-row settings table (id is pinned to 1 by the CHECK constraint),
-- same pattern as resource_limits (000011) and egress_settings (000014).
-- timeout_seconds is how long a detached terminal session (FEAT-015) may
-- sit with no attached WebSocket before AgentService's background sweep
-- auto-terminates it (FEAT-022). 0 means "never" - sessions persist until
-- explicitly terminated. Default is Never on both fresh installs and
-- existing ones.
INSERT OR IGNORE INTO idle_timeout_settings (id, timeout_seconds) VALUES (1, 0);
