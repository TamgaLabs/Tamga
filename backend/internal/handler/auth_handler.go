package handler

import (
	"encoding/json"
	"net/http"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type setupRequest struct {
	Password string `json:"password"`
}

type loginRequest struct {
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}
	user, err := h.auth.Setup(req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	json.NewEncoder(w).Encode(user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}
	token, err := h.auth.Login(req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(loginResponse{Token: token})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(int64)
	json.NewEncoder(w).Encode(map[string]int64{"user_id": userID})
}

func (h *AuthHandler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	setup, err := h.auth.IsSetup()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"setup": setup})
}
