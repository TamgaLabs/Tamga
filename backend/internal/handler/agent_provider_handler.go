package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type AgentProviderHandler struct {
	svc *service.AgentProviderService
}

func NewAgentProviderHandler(svc *service.AgentProviderService) *AgentProviderHandler {
	return &AgentProviderHandler{svc: svc}
}

func (h *AgentProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	providers, err := h.svc.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if providers == nil {
		providers = []*domain.AgentProvider{}
	}
	json.NewEncoder(w).Encode(providers)
}

func (h *AgentProviderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	provider, err := h.svc.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(provider)
}

func (h *AgentProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var p domain.AgentProvider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if p.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if p.Type != domain.ProviderTypeDocker {
		http.Error(w, "type must be 'docker'", http.StatusBadRequest)
		return
	}

	if err := h.svc.Create(&p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func (h *AgentProviderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	var p domain.AgentProvider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	p.ID = id

	if err := h.svc.Update(&p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(p)
}

func (h *AgentProviderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	if err := h.svc.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
