package service

import (
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// ResourceLimitService owns the single global default CPU/memory limit
// applied to every agent sandbox container at creation time (see
// FEAT-007). Unlike WhitelistService/ApiKeyService there's exactly one
// setting here (Get/Set), not a list to CRUD.
type ResourceLimitService struct {
	db *sqlite.DB
}

func NewResourceLimitService(db *sqlite.DB) *ResourceLimitService {
	return &ResourceLimitService{db: db}
}

func (s *ResourceLimitService) Get() (*domain.ResourceLimit, error) {
	return s.db.GetResourceLimit()
}

// Set overwrites the global default. Both values must be strictly positive
// so a sandbox default can never be configured as "unlimited" - per
// FEAT-007 no sandbox should ever be created without a limit.
func (s *ResourceLimitService) Set(memoryBytes, nanoCPUs int64) (*domain.ResourceLimit, error) {
	if memoryBytes <= 0 {
		return nil, fmt.Errorf("memory_bytes must be greater than 0")
	}
	if nanoCPUs <= 0 {
		return nil, fmt.Errorf("nano_cpus must be greater than 0")
	}
	rl := &domain.ResourceLimit{MemoryBytes: memoryBytes, NanoCPUs: nanoCPUs}
	if err := s.db.UpdateResourceLimit(rl); err != nil {
		return nil, err
	}
	return rl, nil
}
