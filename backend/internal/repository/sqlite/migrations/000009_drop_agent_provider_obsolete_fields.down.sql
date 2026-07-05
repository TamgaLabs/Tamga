ALTER TABLE agent_providers ADD COLUMN command TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_providers ADD COLUMN endpoint TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_providers ADD COLUMN auth_token TEXT NOT NULL DEFAULT '';
