package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// GitCredentialHandler exposes the single global git credential (see
// FEAT-008) - a GET/PUT/DELETE settings endpoint, not list CRUD, same
// shape as ResourceLimitHandler.
type GitCredentialHandler struct {
	svc *service.GitCredentialService
}

func NewGitCredentialHandler(svc *service.GitCredentialService) *GitCredentialHandler {
	return &GitCredentialHandler{svc: svc}
}

func (h *GitCredentialHandler) Get(w http.ResponseWriter, r *http.Request) {
	c, err := h.svc.Get()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(c)
}

func (h *GitCredentialHandler) Set(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		Username string `json:"username,omitempty"`
		Token    string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	c, err := h.svc.Set(req.Provider, req.Username, req.Token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(c)
}

func (h *GitCredentialHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
