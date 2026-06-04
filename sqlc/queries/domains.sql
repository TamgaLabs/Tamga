-- name: CreateDomain :one
INSERT INTO domains (id, project_id, domain, created_at, updated_at)
VALUES (gen_random_uuid(), $1, $2, NOW(), NOW())
RETURNING *;

-- name: ListDomainsByProject :many
SELECT d.* FROM domains d
JOIN projects p ON p.id = d.project_id
WHERE d.project_id = $1 AND p.user_id = $2
ORDER BY d.created_at DESC;

-- name: DeleteDomain :one
DELETE FROM domains WHERE id = $1 RETURNING id;
