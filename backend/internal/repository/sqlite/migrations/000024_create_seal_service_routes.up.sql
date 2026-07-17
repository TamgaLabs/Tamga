CREATE TABLE seal_service_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seal_id INTEGER NOT NULL REFERENCES seals(id) ON DELETE CASCADE,
    service_id INTEGER NOT NULL REFERENCES seal_services(id) ON DELETE CASCADE,
    domain TEXT NOT NULL COLLATE NOCASE UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(seal_id, service_id, domain)
);

CREATE INDEX idx_seal_service_routes_service ON seal_service_routes(seal_id, service_id);
