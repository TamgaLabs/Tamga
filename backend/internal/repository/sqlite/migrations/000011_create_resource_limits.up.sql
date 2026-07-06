CREATE TABLE IF NOT EXISTS resource_limits (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    memory_bytes INTEGER NOT NULL,
    nano_cpus INTEGER NOT NULL
);

-- Single-row settings table (id is pinned to 1 by the CHECK constraint).
-- Sensible default: 1 GiB memory, 1 CPU per sandbox. Overridable from
-- Settings (see FEAT-007) - applied automatically to every sandbox
-- container at creation time so no sandbox is ever created unlimited.
INSERT OR IGNORE INTO resource_limits (id, memory_bytes, nano_cpus) VALUES
(1, 1073741824, 1000000000);
