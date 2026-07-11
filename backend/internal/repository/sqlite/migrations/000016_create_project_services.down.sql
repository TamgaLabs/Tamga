DROP TABLE IF EXISTS project_service_containers;
ALTER TABLE projects DROP COLUMN exposed_service;
ALTER TABLE projects DROP COLUMN compose_yaml;
