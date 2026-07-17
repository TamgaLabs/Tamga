package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/handler"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestSealRoutesCreateEmptySealAndExcludeProjectRoutes(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "tamga.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	sealHandler := handler.NewSealHandler(service.NewSealService(db, config.Config{DataDir: t.TempDir()}))
	r := New(nil, nil, sealHandler, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, func(next http.Handler) http.Handler {
		return next
	})

	request := httptest.NewRequest(http.MethodPost, "/api/seals", bytes.NewBufferString(`{"name":"empty seal","domain":"empty.example.test"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("POST /api/seals status = %d, want %d: %s", response.Code, http.StatusCreated, response.Body.String())
	}
	var seal domain.Seal
	if err := json.NewDecoder(response.Body).Decode(&seal); err != nil {
		t.Fatalf("decode seal response: %v", err)
	}
	if seal.ID == 0 || seal.Name != "empty seal" || seal.Domain != "empty.example.test" {
		t.Fatalf("unexpected seal response: %+v", seal)
	}
	if seal.SourceType != domain.SourceTypeEmpty || seal.Status != domain.ProjectStatusConfiguring {
		t.Fatalf("empty Seal lifecycle = source_type %q status %q, want %q and %q", seal.SourceType, seal.Status, domain.SourceTypeEmpty, domain.ProjectStatusConfiguring)
	}

	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/seals", bytes.NewBufferString(`{"domain":"empty.example.test"}`))
	invalidResponse := httptest.NewRecorder()
	r.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/seals without name status = %d, want %d", invalidResponse.Code, http.StatusBadRequest)
	}

	legacyRequest := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	legacyResponse := httptest.NewRecorder()
	r.ServeHTTP(legacyResponse, legacyRequest)
	if legacyResponse.Code != http.StatusNotFound {
		t.Fatalf("GET /api/projects status = %d, want %d", legacyResponse.Code, http.StatusNotFound)
	}
}
