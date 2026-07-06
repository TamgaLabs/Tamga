package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// ResourceLimitHandler exposes the global default agent sandbox CPU/memory
// limit (see FEAT-007). A single GET/PUT settings endpoint, not list CRUD -
// there's only ever one default.
type ResourceLimitHandler struct {
	svc *service.ResourceLimitService
}

func NewResourceLimitHandler(svc *service.ResourceLimitService) *ResourceLimitHandler {
	return &ResourceLimitHandler{svc: svc}
}

func (h *ResourceLimitHandler) Get(w http.ResponseWriter, r *http.Request) {
	rl, err := h.svc.Get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(rl)
}

func (h *ResourceLimitHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemoryBytes int64 `json:"memory_bytes"`
		NanoCPUs    int64 `json:"nano_cpus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	rl, err := h.svc.Set(req.MemoryBytes, req.NanoCPUs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(rl)
}
