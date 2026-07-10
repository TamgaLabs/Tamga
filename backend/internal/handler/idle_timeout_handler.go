package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// IdleTimeoutHandler exposes the global detached-terminal-session idle
// timeout setting (see FEAT-022). A single GET/PUT settings endpoint, not
// list CRUD - there's only ever one value.
type IdleTimeoutHandler struct {
	svc *service.IdleTimeoutService
}

func NewIdleTimeoutHandler(svc *service.IdleTimeoutService) *IdleTimeoutHandler {
	return &IdleTimeoutHandler{svc: svc}
}

func (h *IdleTimeoutHandler) Get(w http.ResponseWriter, r *http.Request) {
	it, err := h.svc.Get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(it)
}

func (h *IdleTimeoutHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TimeoutSeconds int64 `json:"timeout_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	it, err := h.svc.Set(req.TimeoutSeconds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(it)
}
