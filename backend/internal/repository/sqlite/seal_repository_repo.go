package sqlite

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func (db *DB) CreateSealRepository(repository *domain.SealRepository) error {
	result, err := db.Exec(`INSERT INTO seal_repositories
		(seal_id, display_name, remote_url, branch, workspace_path, status, error_summary)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		repository.SealID, repository.DisplayName, repository.RemoteURL, repository.Branch,
		repository.WorkspacePath, repository.Status, repository.ErrorSummary)
	if err != nil {
		return fmt.Errorf("create seal repository: %w", err)
	}
	repository.ID, _ = result.LastInsertId()
	return nil
}

func (db *DB) FindSealRepository(sealID, repositoryID int64) (*domain.SealRepository, error) {
	repository := &domain.SealRepository{}
	err := db.QueryRow(`SELECT id, seal_id, display_name, remote_url, branch, workspace_path,
		status, error_summary, created_at, updated_at FROM seal_repositories
		WHERE seal_id = ? AND id = ?`, sealID, repositoryID).Scan(
		&repository.ID, &repository.SealID, &repository.DisplayName, &repository.RemoteURL,
		&repository.Branch, &repository.WorkspacePath, &repository.Status, &repository.ErrorSummary,
		&repository.CreatedAt, &repository.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("find seal repository: %w", err)
	}
	return repository, nil
}

func (db *DB) ListSealRepositories(sealID int64) ([]*domain.SealRepository, error) {
	rows, err := db.Query(`SELECT id, seal_id, display_name, remote_url, branch, workspace_path,
		status, error_summary, created_at, updated_at FROM seal_repositories
		WHERE seal_id = ? ORDER BY id`, sealID)
	if err != nil {
		return nil, fmt.Errorf("list seal repositories: %w", err)
	}
	defer rows.Close()

	var repositories []*domain.SealRepository
	for rows.Next() {
		repository := &domain.SealRepository{}
		if err := rows.Scan(&repository.ID, &repository.SealID, &repository.DisplayName,
			&repository.RemoteURL, &repository.Branch, &repository.WorkspacePath,
			&repository.Status, &repository.ErrorSummary, &repository.CreatedAt, &repository.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan seal repository: %w", err)
		}
		repositories = append(repositories, repository)
	}
	return repositories, rows.Err()
}

func (db *DB) UpdateSealRepository(repository *domain.SealRepository) error {
	_, err := db.Exec(`UPDATE seal_repositories SET display_name=?, remote_url=?, branch=?,
		workspace_path=?, status=?, error_summary=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND seal_id=?`, repository.DisplayName, repository.RemoteURL, repository.Branch,
		repository.WorkspacePath, repository.Status, repository.ErrorSummary, repository.ID, repository.SealID)
	if err != nil {
		return fmt.Errorf("update seal repository: %w", err)
	}
	return nil
}

func (db *DB) DeleteSealRepository(sealID, repositoryID int64) error {
	if _, err := db.Exec("DELETE FROM seal_repositories WHERE seal_id = ? AND id = ?", sealID, repositoryID); err != nil {
		return fmt.Errorf("delete seal repository: %w", err)
	}
	return nil
}
