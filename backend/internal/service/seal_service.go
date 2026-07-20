package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	traefikrepo "github.com/TamgaLabs/Tamga/backend/internal/repository/traefik"
)

// SealService owns lifecycle operations that are valid before a Seal has any
// repositories or deployment configuration.
type SealService struct {
	db      *sqlite.DB
	cfg     config.Config
	runtime projectRuntime
	routes  projectRoutePublisher
}

// NewSealService accepts an optional Docker client so configuration-only
// callers remain usable when Docker is unavailable. Runtime operations fail
// closed until the API bootstrap supplies that client.
func NewSealService(db *sqlite.DB, cfg config.Config, docker ...*dockerclient.Client) *SealService {
	svc := &SealService{db: db, cfg: cfg}
	if len(docker) > 0 && docker[0] != nil {
		svc.runtime = dockerProjectRuntime{client: docker[0]}
	}
	return svc
}

// SetRoutePublisher installs the project-scoped dynamic-route publisher.
func (s *SealService) SetRoutePublisher(publisher *traefikrepo.Client) {
	s.routes = publisher
}

type CreateSealRequest struct {
	Name string `json:"name"`
}

// List returns all persisted Seals for authenticated API consumers.
func (s *SealService) List(_ context.Context) ([]*domain.Seal, error) {
	return s.db.ListSeals()
}

// Find returns one persisted Seal by ID.
func (s *SealService) Find(_ context.Context, sealID int64) (*domain.Seal, error) {
	return s.db.FindSeal(sealID)
}

// Create persists an unconfigured Seal and establishes its owned workspace.
// It deliberately performs no repository, Docker, or deployment operation:
// those begin only after a repository or configuration is explicitly added.
func (s *SealService) Create(_ context.Context, req CreateSealRequest) (*domain.Seal, error) {
	seal := &domain.Seal{Name: req.Name}
	if err := s.db.CreateSeal(seal); err != nil {
		return nil, fmt.Errorf("create seal: %w", err)
	}

	workspace := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", seal.ID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("create seal workspace: %w", err)
	}
	return seal, nil
}
