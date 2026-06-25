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
	agentHandler *handler.AgentHandler,
	containerHandler *handler.ContainerHandler,
	codeHandler *handler.CodeHandler,
	agentProviderHandler *handler.AgentProviderHandler,
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
			r.Post("/projects/{id}/agent/chat", agentHandler.Chat)
			r.Post("/projects/{id}/agent/chat/stream", agentHandler.ChatStream)
			r.Get("/projects/{id}/agent/tasks", agentHandler.ListTasks)
			r.Get("/projects/{id}/agent/tasks/{taskId}", agentHandler.GetTask)

			// Code agent routes
			r.Post("/code/{projectID}/agent/chat", codeHandler.Chat)
			r.Post("/code/{projectID}/agent/chat/stream", codeHandler.ChatStream)
			r.Get("/code/{projectID}/agent/tasks", codeHandler.ListTasks)
			r.Get("/code/{projectID}/agent/tasks/{taskId}", codeHandler.GetTask)
			r.Get("/code/{projectID}/agent/status", codeHandler.AgentStatus)
			r.Post("/code/{projectID}/agent/start", codeHandler.StartAgent)
			r.Post("/code/{projectID}/agent/stop", codeHandler.StopAgent)
			r.Get("/code/{projectID}/agent/sessions", codeHandler.ListSessions)
			r.Post("/code/{projectID}/agent/sessions", codeHandler.CreateSession)
			r.Put("/code/{projectID}/agent/sessions/{sessionId}", codeHandler.RenameSession)
			r.Delete("/code/{projectID}/agent/sessions/{sessionId}", codeHandler.DeleteSession)
			r.Get("/code/{projectID}/agent/sessions/{sessionId}/tasks", codeHandler.ListSessionTasks)

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

			// Code
			r.Get("/code/projects", codeHandler.ListCodebases)
			r.Get("/code/{projectID}/tree", codeHandler.FileTree)
			r.Get("/code/{projectID}/file", codeHandler.ReadFile)
			r.Put("/code/{projectID}/file", codeHandler.WriteFile)

		})
	})

	return r
}
