-- name: CreateEnvVar :one
INSERT INTO env_vars (id, project_id, key, value, created_at, updated_at)
VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
RETURNING *;

-- name: ListEnvVarsByProject :many
SELECT e.* FROM env_vars e
JOIN projects p ON p.id = e.project_id
WHERE e.project_id = $1 AND p.user_id = $2
ORDER BY e.key ASC;

-- name: UpdateEnvVar :one
UPDATE env_vars
SET key = $2, value = $3, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteEnvVar :one
DELETE FROM env_vars WHERE id = $1 RETURNING id;
