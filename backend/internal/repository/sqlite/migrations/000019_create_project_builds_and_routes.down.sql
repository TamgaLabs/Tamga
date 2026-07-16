DROP TABLE IF EXISTS project_routes;
ALTER TABLE projects DROP COLUMN build_revision;
ALTER TABLE projects DROP COLUMN config_revision;
