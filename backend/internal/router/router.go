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
	terminalHandler *handler.TerminalHandler,
	containerHandler *handler.ContainerHandler,
	codeHandler *handler.CodeHandler,
	agentProviderHandler *handler.AgentProviderHandler,
	apiKeyHandler *handler.ApiKeyHandler,
	whitelistHandler *handler.WhitelistHandler,
	resourceLimitHandler *handler.ResourceLimitHandler,
	gitCredentialHandler *handler.GitCredentialHandler,
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

			// Projects
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
			r.Get("/projects/{id}/agent/terminal", terminalHandler.Serve)

			// Agent Providers
			r.Get("/agent-providers", agentProviderHandler.List)
			r.Get("/agent-providers/{id}", agentProviderHandler.Get)
			r.Post("/agent-providers", agentProviderHandler.Create)
			r.Put("/agent-providers/{id}", agentProviderHandler.Update)
			r.Delete("/agent-providers/{id}", agentProviderHandler.Delete)

			// System / Docker containers
			r.Get("/system/containers", containerHandler.List)
			r.Get("/system/containers/{id}", containerHandler.Inspect)
			r.Post("/system/containers/{id}/start", containerHandler.Start)
			r.Post("/system/containers/{id}/stop", containerHandler.Stop)
			r.Post("/system/containers/{id}/restart", containerHandler.Restart)
			r.Delete("/system/containers/{id}", containerHandler.Remove)
			r.Get("/system/containers/{id}/logs", containerHandler.Logs)
			r.Get("/system/containers/{id}/stats", containerHandler.Stats)
			r.Put("/system/containers/{id}/resources", containerHandler.UpdateResources)
			r.Post("/system/prune", containerHandler.Prune)
			r.Get("/system/info", containerHandler.Info)

			// API Keys
			r.Get("/system/api-keys", apiKeyHandler.List)
			r.Post("/system/api-keys", apiKeyHandler.Set)
			r.Delete("/system/api-keys/{id}", apiKeyHandler.Delete)

			// Agent egress whitelist
			r.Get("/system/egress-whitelist", whitelistHandler.List)
			r.Post("/system/egress-whitelist", whitelistHandler.Create)
			r.Delete("/system/egress-whitelist/{id}", whitelistHandler.Delete)

			// Agent sandbox default resource limit
			r.Get("/system/resource-limits", resourceLimitHandler.Get)
			r.Put("/system/resource-limits", resourceLimitHandler.Update)

			// Global git credential (clone/pull + sandbox commit/push)
			r.Get("/system/git-credential", gitCredentialHandler.Get)
			r.Put("/system/git-credential", gitCredentialHandler.Set)
			r.Delete("/system/git-credential", gitCredentialHandler.Delete)

			// Code
			r.Get("/code/projects", codeHandler.ListCodebases)
			r.Get("/code/{projectID}/tree", codeHandler.FileTree)
			r.Get("/code/{projectID}/file", codeHandler.ReadFile)
			r.Put("/code/{projectID}/file", codeHandler.WriteFile)

		})
	})

	return r
}
