package service

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/google/uuid"
)

type ApiKeyService struct {
	db      *sqlite.DB
	authKey []byte
}

func NewApiKeyService(db *sqlite.DB, jwtSecret string) *ApiKeyService {
	h := sha256.Sum256([]byte(jwtSecret))
	return &ApiKeyService{db: db, authKey: h[:]}
}

func (s *ApiKeyService) Set(provider, key, label string) (*domain.ApiKeyResponse, error) {
	existing, err := s.db.FindApiKeyByProvider(provider)

	var id string
	if existing != nil {
		id = existing.ID
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		// A real DB error occurred (not just "not found")
		return nil, fmt.Errorf("check existing api key: %w", err)
	}

	if id == "" {
		id = uuid.New().String()[:12]
	}

	enc, err := encryptSecret(s.authKey, key)
	if err != nil {
		return nil, fmt.Errorf("encrypt key: %w", err)
	}

	k := &domain.ApiKey{
		ID:       id,
		Provider: provider,
		KeyEnc:   enc,
		Label:    label,
	}

	if existing != nil {
		if err := s.db.DeleteApiKey(id); err != nil {
			return nil, fmt.Errorf("delete old key: %w", err)
		}
	}

	if err := s.db.CreateApiKey(k); err != nil {
		return nil, fmt.Errorf("save api key: %w", err)
	}

	return s.toResponse(k), nil
}

func (s *ApiKeyService) List() ([]*domain.ApiKeyResponse, error) {
	keys, err := s.db.ListApiKeys()
	if err != nil {
		return nil, err
	}
	resp := make([]*domain.ApiKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, s.toResponse(k))
	}
	return resp, nil
}

func (s *ApiKeyService) Get(id string) (*domain.ApiKeyResponse, error) {
	k, err := s.db.FindApiKey(id)
	if err != nil {
		return nil, err
	}
	return s.toResponse(k), nil
}

func (s *ApiKeyService) Delete(id string) error {
	return s.db.DeleteApiKey(id)
}

func (s *ApiKeyService) GetAllAsEnv() (map[string]string, error) {
	keys, err := s.db.ListApiKeys()
	if err != nil {
		return nil, err
	}
	env := make(map[string]string)
	for _, k := range keys {
		envVar, ok := domain.ProviderEnvVars[k.Provider]
		if !ok {
			continue
		}
		dec, err := decryptSecret(s.authKey, k.KeyEnc)
		if err != nil {
			continue
		}
		env[envVar] = dec
	}
	return env, nil
}

func (s *ApiKeyService) toResponse(k *domain.ApiKey) *domain.ApiKeyResponse {
	return &domain.ApiKeyResponse{
		ID:        k.ID,
		Provider:  k.Provider,
		Label:     k.Label,
		HasKey:    k.KeyEnc != "",
		CreatedAt: k.CreatedAt,
		UpdatedAt: k.UpdatedAt,
	}
}
