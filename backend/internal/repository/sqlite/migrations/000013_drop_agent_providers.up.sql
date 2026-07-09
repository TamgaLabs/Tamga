DROP TABLE agent_providers;

ALTER TABLE projects DROP COLUMN agent_provider_id;

DROP TABLE IF EXISTS api_keys;
