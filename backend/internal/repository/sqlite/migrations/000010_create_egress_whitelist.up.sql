CREATE TABLE IF NOT EXISTS egress_whitelist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Sensible defaults covering the major AI provider APIs. Extensible from
-- Settings; these just seed the initial list so the sandbox egress proxy
-- (see FEAT-006) is useful out of the box.
INSERT OR IGNORE INTO egress_whitelist (domain) VALUES
('api.anthropic.com'),
('api.openai.com'),
('generativelanguage.googleapis.com');
