package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
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

func (h *SealHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	repositories, err := h.svc.ListRepositories(r.Context(), sealID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(repositories)
}

func (h *SealHandler) CreateRepository(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	var req service.CreateSealRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	repository, err := h.svc.CreateRepository(r.Context(), sealID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(repository)
}

func (h *SealHandler) RefreshRepository(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	repositoryID, ok := repositoryIDFromRequest(w, r)
	if !ok {
		return
	}
	repository, err := h.svc.RefreshRepository(r.Context(), sealID, repositoryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(repository)
}

func (h *SealHandler) DeleteRepository(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	repositoryID, ok := repositoryIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteRepository(r.Context(), sealID, repositoryID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SealHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	services, err := h.svc.ListServices(r.Context(), sealID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(services)
}

func (h *SealHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	var req service.CreateSealServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	created, err := h.svc.CreateService(r.Context(), sealID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (h *SealHandler) ListServiceRoutes(w http.ResponseWriter, r *http.Request) {
	sealID, serviceID, ok := sealServiceIDsFromRequest(w, r)
	if !ok {
		return
	}
	routes, err := h.svc.ListServiceRoutes(r.Context(), sealID, serviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if routes == nil {
		routes = []*domain.SealServiceRoute{}
	}
	json.NewEncoder(w).Encode(routes)
}

func (h *SealHandler) CreateServiceRoute(w http.ResponseWriter, r *http.Request) {
	sealID, serviceID, ok := sealServiceIDsFromRequest(w, r)
	if !ok {
		return
	}
	var req service.CreateSealServiceRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	route, err := h.svc.AddServiceRoute(r.Context(), sealID, serviceID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidSealServiceRoute):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, service.ErrSealServiceRouteConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusNotFound)
		}
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(route)
}

func (h *SealHandler) DeleteServiceRoute(w http.ResponseWriter, r *http.Request) {
	sealID, serviceID, ok := sealServiceIDsFromRequest(w, r)
	if !ok {
		return
	}
	routeID, err := strconv.ParseInt(chi.URLParam(r, "routeID"), 10, 64)
	if err != nil || routeID <= 0 {
		http.Error(w, "invalid route id", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteServiceRoute(r.Context(), sealID, serviceID, routeID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SealHandler) Configuration(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	configuration, err := h.svc.Configuration(r.Context(), sealID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(configuration)
}

func (h *SealHandler) SaveConfiguration(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	var req service.SaveSealConfigurationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	configuration, err := h.svc.SaveConfiguration(r.Context(), sealID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(configuration)
}

// Deploy starts a validated direct Seal configuration. Generated
// configurations remain fail-closed until their Seal-native build lifecycle
// has produced images; no legacy ProjectService deployment path is used.
func (h *SealHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := h.svc.Deploy(r.Context(), sealID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func sealIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "sealID"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid seal id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func repositoryIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "repositoryID"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid repository id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func sealServiceIDsFromRequest(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return 0, 0, false
	}
	serviceID, err := strconv.ParseInt(chi.URLParam(r, "serviceID"), 10, 64)
	if err != nil || serviceID <= 0 {
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return 0, 0, false
	}
	return sealID, serviceID, true
}
