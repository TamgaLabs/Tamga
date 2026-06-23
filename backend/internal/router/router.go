package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/TamgaLabs/Tamga/backend/internal/handler"
)

func New(
	authHandler *handler.AuthHandler,
	systemHandler *handler.SystemHandler,
	projectHandler *handler.ProjectHandler,
	authMiddleware func(http.Handler) http.Handler,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/health", systemHandler.Health)

	r.Route("/api", func(r chi.Router) {
		r.Get("/auth/status", authHandler.SetupStatus)
		r.Post("/auth/setup", authHandler.Setup)
		r.Post("/auth/login", authHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Get("/auth/me", authHandler.Me)

			r.Get("/projects", projectHandler.List)
			r.Post("/projects", projectHandler.Create)
			r.Get("/projects/{id}", projectHandler.Get)
			r.Put("/projects/{id}", projectHandler.Update)
			r.Delete("/projects/{id}", projectHandler.Delete)
			r.Post("/projects/{id}/restart", projectHandler.Restart)
			r.Get("/projects/{id}/logs", projectHandler.Logs)
			r.Get("/projects/{id}/deployments", projectHandler.ListDeployments)
			r.Get("/projects/{id}/env-vars", projectHandler.ListEnvVars)
			r.Post("/projects/{id}/env-vars", projectHandler.CreateEnvVar)
			r.Delete("/projects/{id}/env-vars/{envVarId}", projectHandler.DeleteEnvVar)
		})
	})

	return r
}
