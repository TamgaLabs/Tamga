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
	runtime sealRuntime
	routes  sealRoutePublisher
}

// NewSealService accepts an optional Docker client so configuration-only
// callers remain usable when Docker is unavailable. Runtime operations fail
// closed until the API bootstrap supplies that client.
func NewSealService(db *sqlite.DB, cfg config.Config, docker ...*dockerclient.Client) *SealService {
	svc := &SealService{db: db, cfg: cfg}
	if len(docker) > 0 && docker[0] != nil {
		svc.runtime = dockerSealRuntime{client: docker[0]}
	}
	return svc
}

// SetRoutePublisher gives the Seal runtime ownership of its dynamic proxy
// configuration without coupling configuration-only callers to Traefik.
func (s *SealService) SetRoutePublisher(publisher *traefikrepo.Client) {
	s.routes = publisher
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
		Name:            req.Name,
		SourceType:      domain.SourceTypeEmpty,
		Domain:          req.Domain,
		Status:          domain.ProjectStatusConfiguring,
		Branch:          "main",
		ConfigAuthority: "generated",
	}
	if err := s.db.CreateSeal(seal); err != nil {
		return nil, fmt.Errorf("create seal: %w", err)
	}

	workspace := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", seal.ID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return nil, fmt.Errorf("create seal workspace: %w", err)
	}
	if err := s.writeGeneratedCompose(seal.ID, "services: {}\n", false); err != nil {
		return nil, err
	}
	return seal, nil
}
