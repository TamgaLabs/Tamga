package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// SealHandler exposes the lifecycle operations available for Seals before
// sources, configuration, and deployment are added.
type SealHandler struct {
	svc *service.SealService
}

func NewSealHandler(svc *service.SealService) *SealHandler {
	return &SealHandler{svc: svc}
}

func (h *SealHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req service.CreateSealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	seal, err := h.svc.Create(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(seal)
}
