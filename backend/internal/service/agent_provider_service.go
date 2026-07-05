package service

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/google/uuid"
)

type AgentProviderService struct {
	db *sqlite.DB
}

func NewAgentProviderService(db *sqlite.DB) *AgentProviderService {
	return &AgentProviderService{db: db}
}

func (s *AgentProviderService) List() ([]*domain.AgentProvider, error) {
	return s.db.ListAgentProviders()
}

func (s *AgentProviderService) Get(id string) (*domain.AgentProvider, error) {
	return s.db.FindAgentProvider(id)
}

func (s *AgentProviderService) Create(p *domain.AgentProvider) error {
	if p.ID == "" {
		p.ID = uuid.New().String()[:12]
	}
	return s.db.CreateAgentProvider(p)
}

func (s *AgentProviderService) Update(p *domain.AgentProvider) error {
	existing, err := s.db.FindAgentProvider(p.ID)
	if err != nil {
		return fmt.Errorf("provider not found: %w", err)
	}
	if existing.IsDefault {
		return fmt.Errorf("cannot modify default provider")
	}
	return s.db.UpdateAgentProvider(p)
}

func (s *AgentProviderService) Delete(id string) error {
	return s.db.DeleteAgentProvider(id)
}

func (s *AgentProviderService) ResolveProvider(providerID string) (*domain.AgentProvider, error) {
	if providerID != "" {
		return s.db.FindAgentProvider(providerID)
	}
	return s.db.FindDefaultProvider()
}
