package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// TopologyHandler serves the infrastructure topology graph API.
type TopologyHandler struct {
	svc *service.TopologyService
}

func NewTopologyHandler(svc *service.TopologyService) *TopologyHandler {
	return &TopologyHandler{svc: svc}
}

// System handles GET /api/system/topology - the global infra graph.
func (h *TopologyHandler) System(w http.ResponseWriter, r *http.Request) {
	topology, err := h.svc.GetSystemTopology(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(topology)
}

// Project handles GET /api/projects/{id}/topology - per-project topology.
func (h *TopologyHandler) Project(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	topology, err := h.svc.GetProjectTopology(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(topology)
}
