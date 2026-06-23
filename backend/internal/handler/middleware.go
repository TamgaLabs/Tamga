package handler

import (
	"context"
	"net/http"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

type ContextKey string

const UserIDKey ContextKey = "user_id"

func AuthMiddleware(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := r.Header.Get("Authorization")
			if tokenStr == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}
			if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
				tokenStr = tokenStr[7:]
			}
			userID, err := auth.ValidateToken(tokenStr)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
