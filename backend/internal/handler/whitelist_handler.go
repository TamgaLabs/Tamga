package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// WhitelistHandler exposes CRUD for the agent sandbox egress whitelist
// (see FEAT-006). Same small REST-CRUD shape as AgentProviderHandler.
type WhitelistHandler struct {
	svc *service.WhitelistService
}

func NewWhitelistHandler(svc *service.WhitelistService) *WhitelistHandler {
	return &WhitelistHandler{svc: svc}
}

func (h *WhitelistHandler) List(w http.ResponseWriter, r *http.Request) {
	domains, err := h.svc.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if domains == nil {
		domains = []*domain.WhitelistDomain{}
	}
	json.NewEncoder(w).Encode(domains)
}

func (h *WhitelistHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Domain == "" {
		http.Error(w, "domain is required", http.StatusBadRequest)
		return
	}

	d, err := h.svc.Add(req.Domain)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			http.Error(w, "domain already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(d)
}

func (h *WhitelistHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.svc.Remove(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
