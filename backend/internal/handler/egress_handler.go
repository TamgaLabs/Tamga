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

// EgressHandler exposes the global egress mode setting and the egress
// blacklist CRUD (see FEAT-016). The existing egress whitelist endpoints
// stay on WhitelistHandler, unchanged.
type EgressHandler struct {
	svc *service.EgressService
}

func NewEgressHandler(svc *service.EgressService) *EgressHandler {
	return &EgressHandler{svc: svc}
}

func (h *EgressHandler) GetMode(w http.ResponseWriter, r *http.Request) {
	mode, err := h.svc.GetMode()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(domain.EgressSettings{Mode: mode})
}

func (h *EgressHandler) SetMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	mode, err := h.svc.SetMode(req.Mode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(domain.EgressSettings{Mode: mode})
}

func (h *EgressHandler) ListBlacklist(w http.ResponseWriter, r *http.Request) {
	domains, err := h.svc.ListBlacklist()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if domains == nil {
		domains = []*domain.BlacklistDomain{}
	}
	json.NewEncoder(w).Encode(domains)
}

func (h *EgressHandler) CreateBlacklist(w http.ResponseWriter, r *http.Request) {
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

	d, err := h.svc.AddBlacklist(req.Domain)
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

func (h *EgressHandler) DeleteBlacklist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.svc.RemoveBlacklist(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
