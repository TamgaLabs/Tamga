package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

type ContextKey string

const UserIDKey ContextKey = "user_id"

func AuthMiddleware(auth *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := r.Header.Get("Authorization")
			if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
				tokenStr = tokenStr[7:]
			}
			if tokenStr == "" && strings.HasSuffix(r.URL.Path, "/agent/terminal") {
				// The browser WebSocket API can't set custom headers, so the
				// terminal endpoint alone falls back to a `token` query
				// param. Scoped to this path specifically (rather than the
				// whole authenticated group) since query params can end up
				// in server/proxy access logs and Referer headers.
				tokenStr = r.URL.Query().Get("token")
			}
			if tokenStr == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
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
