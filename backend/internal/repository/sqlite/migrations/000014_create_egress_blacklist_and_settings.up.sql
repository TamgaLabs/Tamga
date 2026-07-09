CREATE TABLE IF NOT EXISTS egress_blacklist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS egress_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    mode TEXT NOT NULL DEFAULT 'open'
);

-- Single-row settings table (id is pinned to 1 by the CHECK constraint),
-- same pattern as resource_limits (migration 000011). Default mode is
-- 'open' on both fresh installs and existing ones (deliberate: this
-- migration doesn't try to infer 'whitelist' from an existing populated
-- egress_whitelist table - see FEAT-016 Implementation Notes).
INSERT OR IGNORE INTO egress_settings (id, mode) VALUES (1, 'open');
