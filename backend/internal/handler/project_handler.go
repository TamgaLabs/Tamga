package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

type ProjectHandler struct {
	svc *service.ProjectService
}

func NewProjectHandler(svc *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.svc.List(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if projects == nil {
		projects = []*domain.Project{}
	}
	json.NewEncoder(w).Encode(projects)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	project, err := h.svc.Get(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(project)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string            `json:"name"`
		SourceType     domain.SourceType `json:"source_type"`
		RepoURL        string            `json:"repo_url"`
		Branch         string            `json:"branch,omitempty"`
		Domain         string            `json:"domain"`
		ComposeYAML    string            `json:"compose_yaml,omitempty"`
		ExposedService string            `json:"exposed_service,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Domain == "" {
		http.Error(w, "name and domain are required", http.StatusBadRequest)
		return
	}

	// Compose-project create (FEAT-029): parse compose_yaml up front and
	// reject it here, synchronously, rather than letting the async
	// deploy() goroutine (project_service.go) discover a parse error
	// later - the whole point of validating at create time (carried from
	// FEAT-028's review) is that "build: not supported" etc. surfaces
	// immediately in the response, not silently after the project is
	// already sitting in ProjectStatusError. If exposed_service is also
	// set, it must name one of the parsed services - otherwise
	// detectExposedService's override branch (deploy_engine.go) would
	// trust a name that resolves to no container and build a dead route.
	if req.ComposeYAML != "" {
		services, err := service.ParseComposeYAML(req.ComposeYAML)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.ExposedService != "" {
			found := false
			for _, svc := range services {
				if svc.Name == req.ExposedService {
					found = true
					break
				}
			}
			if !found {
				http.Error(w, fmt.Sprintf("exposed_service %q is not a service defined in compose_yaml", req.ExposedService), http.StatusBadRequest)
				return
			}
		}
		if req.SourceType == "" {
			req.SourceType = domain.SourceTypeCompose
		}
	} else {
		if req.SourceType == "" {
			req.SourceType = domain.SourceTypeRemote
		}
		if req.SourceType == domain.SourceTypeRemote && req.RepoURL == "" {
			http.Error(w, "repo_url is required for remote source", http.StatusBadRequest)
			return
		}
	}

	project, err := h.svc.Create(r.Context(), service.CreateProjectRequest{
		Name:           req.Name,
		SourceType:     req.SourceType,
		RepoURL:        req.RepoURL,
		Branch:         req.Branch,
		Domain:         req.Domain,
		ComposeYAML:    req.ComposeYAML,
		ExposedService: req.ExposedService,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(project)
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	// Parse the request with exposed_service field
	var req struct {
		Name           *string            `json:"name,omitempty"`
		SourceType     *domain.SourceType `json:"source_type,omitempty"`
		RepoURL        *string            `json:"repo_url,omitempty"`
		Domain         *string            `json:"domain,omitempty"`
		Branch         *string            `json:"branch,omitempty"`
		ExposedService *string            `json:"exposed_service,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// If exposed_service is being changed, validate it against the project's
	// compose_yaml services. This mirrors the validation in Create (FEAT-029).
	if req.ExposedService != nil {
		project, err := h.svc.Get(r.Context(), id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// Only validate for compose projects
		if project.ComposeYAML != "" {
			services, err := service.ParseComposeYAML(project.ComposeYAML)
			if err != nil {
				// ComposeYAML was already validated at create time, so this
				// should not happen - but treat it as a server error if it does
				http.Error(w, fmt.Sprintf("internal error parsing compose: %s", err), http.StatusInternalServerError)
				return
			}
			found := false
			for _, svc := range services {
				if svc.Name == *req.ExposedService {
					found = true
					break
				}
			}
			if !found {
				http.Error(w, fmt.Sprintf("exposed_service %q is not a service defined in compose_yaml", *req.ExposedService), http.StatusBadRequest)
				return
			}
		}
	}

	updateReq := service.UpdateProjectRequest{
		Name:           req.Name,
		SourceType:     req.SourceType,
		RepoURL:        req.RepoURL,
		Domain:         req.Domain,
		Branch:         req.Branch,
		ExposedService: req.ExposedService,
	}

	project, err := h.svc.Update(r.Context(), id, updateReq)
	if err != nil {
		// Map service errors to appropriate HTTP status codes
		if strings.Contains(err.Error(), "find project") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if strings.Contains(err.Error(), "no running container") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		// Other errors (DB, etc.)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(project)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProjectHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := h.svc.Restart(r.Context(), id); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ProjectHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	logs, err := h.svc.Logs(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"logs": logs})
}

func (h *ProjectHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	deployments, err := h.svc.GetDeployments(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if deployments == nil {
		deployments = []*domain.Deployment{}
	}
	json.NewEncoder(w).Encode(deployments)
}

func (h *ProjectHandler) ListEnvVars(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	envVars, err := h.svc.ListEnvVars(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if envVars == nil {
		envVars = []*domain.EnvVar{}
	}
	json.NewEncoder(w).Encode(envVars)
}

func (h *ProjectHandler) CreateEnvVar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}
	ev, err := h.svc.CreateEnvVar(r.Context(), id, req.Key, req.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ev)
}

func (h *ProjectHandler) DeleteEnvVar(w http.ResponseWriter, r *http.Request) {
	vid, err := strconv.ParseInt(chi.URLParam(r, "envVarId"), 10, 64)
	if err != nil {
		http.Error(w, "invalid env var id", http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteEnvVar(r.Context(), vid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
