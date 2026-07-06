CREATE TABLE IF NOT EXISTS git_credential (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    provider TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    token_enc TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Single-row settings table (id pinned to 1), same shape as
-- resource_limits (migration 000011). An empty token_enc means "not
-- configured yet" - GitCredentialService reports has_token=false and skips
-- credential injection in that case.
INSERT OR IGNORE INTO git_credential (id) VALUES (1);
