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

func (h *SealHandler) List(w http.ResponseWriter, r *http.Request) {
	seals, err := h.svc.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if seals == nil {
		seals = []*domain.Seal{}
	}
	json.NewEncoder(w).Encode(seals)
}

func (h *SealHandler) Get(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	seal, err := h.svc.Find(r.Context(), sealID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(seal)
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

func (h *SealHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	projects, err := h.svc.ListProjects(r.Context(), sealID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if projects == nil {
		projects = []*domain.Project{}
	}
	json.NewEncoder(w).Encode(projects)
}

func (h *SealHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	var req service.CreateSealProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	project, err := h.svc.CreateProject(r.Context(), sealID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(project)
}

func (h *SealHandler) RefreshProject(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	projectID, ok := projectIDFromRequest(w, r)
	if !ok {
		return
	}
	project, err := h.svc.RefreshProject(r.Context(), sealID, projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(project)
}

func (h *SealHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return
	}
	projectID, ok := projectIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteProject(r.Context(), sealID, projectID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SealHandler) ListProjectServices(w http.ResponseWriter, r *http.Request) {
	sealID, projectID, ok := sealProjectIDsFromRequest(w, r)
	if !ok {
		return
	}
	services, err := h.svc.ListProjectServices(r.Context(), sealID, projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if services == nil {
		services = []*domain.Service{}
	}
	json.NewEncoder(w).Encode(services)
}

func (h *SealHandler) CreateProjectService(w http.ResponseWriter, r *http.Request) {
	sealID, projectID, ok := sealProjectIDsFromRequest(w, r)
	if !ok {
		return
	}
	var req service.CreateProjectServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	created, err := h.svc.CreateProjectService(r.Context(), sealID, projectID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

func (h *SealHandler) ListProjectServiceRoutes(w http.ResponseWriter, r *http.Request) {
	sealID, projectID, serviceID, ok := sealProjectServiceIDsFromRequest(w, r)
	if !ok {
		return
	}
	routes, err := h.svc.ListProjectServiceRoutes(r.Context(), sealID, projectID, serviceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if routes == nil {
		routes = []*domain.ServiceRoute{}
	}
	json.NewEncoder(w).Encode(routes)
}

func (h *SealHandler) CreateProjectServiceRoute(w http.ResponseWriter, r *http.Request) {
	sealID, projectID, serviceID, ok := sealProjectServiceIDsFromRequest(w, r)
	if !ok {
		return
	}
	var req service.CreateProjectServiceRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	route, err := h.svc.AddProjectServiceRoute(r.Context(), sealID, projectID, serviceID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidProjectServiceRoute):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, service.ErrProjectServiceRouteConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusNotFound)
		}
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(route)
}

func (h *SealHandler) DeleteProjectServiceRoute(w http.ResponseWriter, r *http.Request) {
	sealID, projectID, serviceID, ok := sealProjectServiceIDsFromRequest(w, r)
	if !ok {
		return
	}
	routeID, err := strconv.ParseInt(chi.URLParam(r, "routeID"), 10, 64)
	if err != nil || routeID <= 0 {
		http.Error(w, "invalid route id", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteProjectServiceRoute(r.Context(), sealID, projectID, serviceID, routeID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SealHandler) ProjectConfiguration(w http.ResponseWriter, r *http.Request) {
	sealID, projectID, ok := sealProjectIDsFromRequest(w, r)
	if !ok {
		return
	}
	configuration, err := h.svc.ProjectConfiguration(r.Context(), sealID, projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(configuration)
}

func sealIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "sealID"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid seal id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func projectIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func sealProjectIDsFromRequest(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	sealID, ok := sealIDFromRequest(w, r)
	if !ok {
		return 0, 0, false
	}
	projectID, ok := projectIDFromRequest(w, r)
	if !ok {
		return 0, 0, false
	}
	return sealID, projectID, true
}

func sealProjectServiceIDsFromRequest(w http.ResponseWriter, r *http.Request) (int64, int64, int64, bool) {
	sealID, projectID, ok := sealProjectIDsFromRequest(w, r)
	if !ok {
		return 0, 0, 0, false
	}
	serviceID, err := strconv.ParseInt(chi.URLParam(r, "serviceID"), 10, 64)
	if err != nil || serviceID <= 0 {
		http.Error(w, "invalid service id", http.StatusBadRequest)
		return 0, 0, 0, false
	}
	return sealID, projectID, serviceID, true
}
