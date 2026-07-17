package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

type CreateSealRepositoryRequest struct {
	DisplayName string `json:"display_name"`
	RemoteURL   string `json:"remote_url"`
	Branch      string `json:"branch,omitempty"`
}

// ListRepositories returns the repository lifecycle state for a Seal.
func (s *SealService) ListRepositories(ctx context.Context, sealID int64) ([]*domain.SealRepository, error) {
	if _, err := s.db.FindSeal(sealID); err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	return s.db.ListSealRepositories(sealID)
}

// CreateRepository adds a repository to an otherwise empty or already
// configured Seal. The checkout is completed before the result is returned so
// callers can directly observe either ready or clone_failed state.
func (s *SealService) CreateRepository(ctx context.Context, sealID int64, req CreateSealRepositoryRequest) (*domain.SealRepository, error) {
	if _, err := s.db.FindSeal(sealID); err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	repository, err := newSealRepository(sealID, req)
	if err != nil {
		return nil, err
	}
	if err := s.db.CreateSealRepository(repository); err != nil {
		return nil, err
	}
	if err := s.refreshRepository(ctx, repository); err != nil {
		return nil, err
	}
	return repository, nil
}

// RefreshRepository clones into a temporary sibling and only replaces a ready
// checkout after the clone has been validated. Failures leave a prior checkout
// untouched and persist only a credential-safe error summary.
func (s *SealService) RefreshRepository(ctx context.Context, sealID, repositoryID int64) (*domain.SealRepository, error) {
	repository, err := s.db.FindSealRepository(sealID, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("find seal repository: %w", err)
	}
	if err := s.refreshRepository(ctx, repository); err != nil {
		return nil, err
	}
	return repository, nil
}

func (s *SealService) DeleteRepository(ctx context.Context, sealID, repositoryID int64) error {
	repository, err := s.db.FindSealRepository(sealID, repositoryID)
	if err != nil {
		return fmt.Errorf("find seal repository: %w", err)
	}
	checkout, err := s.repositoryCheckoutPath(sealID, repository)
	if err != nil {
		return err
	}
	if err := s.db.DeleteSealRepository(sealID, repositoryID); err != nil {
		return err
	}
	if err := os.RemoveAll(checkout); err != nil {
		return fmt.Errorf("remove repository checkout: %w", err)
	}
	return nil
}

func newSealRepository(sealID int64, req CreateSealRepositoryRequest) (*domain.SealRepository, error) {
	name := strings.TrimSpace(req.DisplayName)
	if !safeRepositoryName(name) {
		return nil, fmt.Errorf("repository display_name must be a safe name")
	}
	if strings.TrimSpace(req.RemoteURL) == "" {
		return nil, fmt.Errorf("repository remote_url is required")
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = "main"
	}
	return &domain.SealRepository{
		SealID:        sealID,
		DisplayName:   name,
		RemoteURL:     redactGitURL(strings.TrimSpace(req.RemoteURL)),
		Branch:        branch,
		WorkspacePath: filepath.ToSlash(filepath.Join("repositories", name)),
		Status:        domain.ProjectSourceStatusPending,
	}, nil
}

func safeRepositoryName(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func (s *SealService) repositoryCheckoutPath(sealID int64, repository *domain.SealRepository) (string, error) {
	if repository.SealID != sealID || !safeRepositoryName(repository.DisplayName) {
		return "", fmt.Errorf("invalid repository ownership")
	}
	expected := filepath.ToSlash(filepath.Join("repositories", repository.DisplayName))
	if repository.WorkspacePath != expected {
		return "", fmt.Errorf("invalid repository workspace path")
	}
	return filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", sealID), filepath.FromSlash(expected)), nil
}

func (s *SealService) refreshRepository(ctx context.Context, repository *domain.SealRepository) error {
	checkout, err := s.repositoryCheckoutPath(repository.SealID, repository)
	if err != nil {
		return err
	}
	parent := filepath.Dir(checkout)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create repository parent: %w", err)
	}
	temporary, err := os.MkdirTemp(parent, "."+repository.DisplayName+".tmp-")
	if err != nil {
		return fmt.Errorf("create repository temporary directory: %w", err)
	}
	defer os.RemoveAll(temporary)

	repository.Status, repository.ErrorSummary = domain.ProjectSourceStatusCloning, ""
	if err := s.db.UpdateSealRepository(repository); err != nil {
		return err
	}
	if err := sealGitClone(ctx, repository.RemoteURL, repository.Branch, temporary); err != nil || !validGitCheckout(temporary) {
		repository.Status = domain.ProjectSourceStatusCloneFailed
		repository.ErrorSummary = "unable to refresh repository"
		if updateErr := s.db.UpdateSealRepository(repository); updateErr != nil {
			return fmt.Errorf("record repository refresh failure: %w", updateErr)
		}
		return nil
	}
	if err := replaceRepositoryCheckout(checkout, temporary); err != nil {
		repository.Status = domain.ProjectSourceStatusCloneFailed
		repository.ErrorSummary = "unable to refresh repository"
		if updateErr := s.db.UpdateSealRepository(repository); updateErr != nil {
			return fmt.Errorf("record repository replacement failure: %w", updateErr)
		}
		return nil
	}
	repository.Status, repository.ErrorSummary = domain.ProjectSourceStatusReady, ""
	return s.db.UpdateSealRepository(repository)
}

func sealGitClone(ctx context.Context, remoteURL, branch, destination string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--single-branch", "--depth", "1", remoteURL, destination)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func validGitCheckout(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func replaceRepositoryCheckout(checkout, temporary string) error {
	backup := checkout + ".previous"
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove stale repository backup: %w", err)
	}
	if _, err := os.Stat(checkout); err == nil {
		if err := os.Rename(checkout, backup); err != nil {
			return fmt.Errorf("preserve current repository checkout: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect current repository checkout: %w", err)
	}
	if err := os.Rename(temporary, checkout); err != nil {
		if restoreErr := os.Rename(backup, checkout); restoreErr != nil && !os.IsNotExist(restoreErr) {
			return fmt.Errorf("install refreshed repository: %w (restore previous checkout: %v)", err, restoreErr)
		}
		return fmt.Errorf("install refreshed repository: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous repository checkout: %w", err)
	}
	return nil
}
