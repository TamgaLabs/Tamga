-- name: CreateGitRepository :one
INSERT INTO git_repositories (id, project_id, url, branch, created_at, updated_at)
VALUES (gen_random_uuid(), $1, $2, $3, NOW(), NOW())
RETURNING *;

-- name: GetGitRepositoryByProject :one
SELECT g.* FROM git_repositories g
JOIN projects p ON p.id = g.project_id
WHERE g.project_id = $1 AND p.user_id = $2
LIMIT 1;

-- name: DeleteGitRepository :one
DELETE FROM git_repositories WHERE id = $1 RETURNING id;
