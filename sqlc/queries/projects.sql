-- name: CreateProject :one
INSERT INTO projects (id, name, description, user_id, created_at, updated_at)
VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
RETURNING *;

-- name: GetProjectByID :one
SELECT * FROM projects WHERE id = $1 AND user_id = $2 LIMIT 1;

-- name: ListProjectsByUser :many
SELECT * FROM projects WHERE user_id = $1 ORDER BY created_at DESC;

-- name: UpdateProject :one
UPDATE projects
SET name = $2, description = $3, updated_at = NOW()
WHERE id = $1 AND user_id = $4
RETURNING *;

-- name: DeleteProject :one
DELETE FROM projects WHERE id = $1 AND user_id = $2 RETURNING id;
