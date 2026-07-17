package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

const emptySealComposeYAML = "services: {}\n"

// SealService owns lifecycle operations that are valid before a Seal has any
// repositories or deployment configuration.
type SealService struct {
	db  *sqlite.DB
	cfg config.Config
}

func NewSealService(db *sqlite.DB, cfg config.Config) *SealService {
	return &SealService{db: db, cfg: cfg}
}

type CreateSealRequest struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// Create persists an unconfigured Seal and establishes its owned workspace.
// It deliberately performs no repository, Docker, or deployment operation:
// those begin only after a repository or configuration is explicitly added.
func (s *SealService) Create(_ context.Context, req CreateSealRequest) (*domain.Seal, error) {
	seal := &domain.Seal{
		Name:       req.Name,
		SourceType: domain.SourceTypeEmpty,
		Domain:     req.Domain,
		Status:     domain.ProjectStatusConfiguring,
		Branch:     "main",
	}
	if err := s.db.CreateSeal(seal); err != nil {
		return nil, fmt.Errorf("create seal: %w", err)
	}

	workspace := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", seal.ID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("create seal workspace: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "compose.yaml"), []byte(emptySealComposeYAML), 0644); err != nil {
		return nil, fmt.Errorf("write empty seal compose file: %w", err)
	}
	return seal, nil
}
