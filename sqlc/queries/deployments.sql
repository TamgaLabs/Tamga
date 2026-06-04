-- name: CreateDeployment :one
INSERT INTO deployments (id, project_id, status, created_at, updated_at)
VALUES (gen_random_uuid(), $1, 'pending', NOW(), NOW())
RETURNING *;

-- name: GetDeploymentByID :one
SELECT d.* FROM deployments d
JOIN projects p ON p.id = d.project_id
WHERE d.id = $1 AND p.user_id = $2
LIMIT 1;

-- name: ListDeploymentsByProject :many
SELECT d.* FROM deployments d
JOIN projects p ON p.id = d.project_id
WHERE d.project_id = $1 AND p.user_id = $2
ORDER BY d.created_at DESC;

-- name: UpdateDeploymentStatus :one
UPDATE deployments
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateDeploymentDetails :one
UPDATE deployments
SET status = $2, commit_sha = $3, commit_message = $4, image_tag = $5, container_id = $6, domain = $7, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: GetLatestDeploymentByProject :one
SELECT d.* FROM deployments d
JOIN projects p ON p.id = d.project_id
WHERE d.project_id = $1 AND p.user_id = $2
ORDER BY d.created_at DESC
LIMIT 1;

-- name: CreateDeploymentLog :one
INSERT INTO deployment_logs (id, deployment_id, stream, message, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, NOW())
RETURNING *;

-- name: ListDeploymentLogs :many
SELECT * FROM deployment_logs
WHERE deployment_id = $1
ORDER BY created_at ASC;
