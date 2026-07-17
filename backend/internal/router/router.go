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
	sealHandler *handler.SealHandler,
	terminalHandler *handler.TerminalHandler,
	containerHandler *handler.ContainerHandler,
	codeHandler *handler.CodeHandler,
	whitelistHandler *handler.WhitelistHandler,
	egressHandler *handler.EgressHandler,
	resourceLimitHandler *handler.ResourceLimitHandler,
	idleTimeoutHandler *handler.IdleTimeoutHandler,
	gitCredentialHandler *handler.GitCredentialHandler,
	metricsHandler *handler.MetricsHandler,
	topologyHandler *handler.TopologyHandler,
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

			// Seals
			r.Post("/seals", sealHandler.Create)

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

			// Metrics query API (FEAT-032) - project + global panels.
			r.Get("/system/metrics", metricsHandler.System)

			// Topology API (FEAT-036) - infra graph
			r.Get("/system/topology", topologyHandler.System)

			// Agent egress whitelist
			r.Get("/system/egress-whitelist", whitelistHandler.List)
			r.Post("/system/egress-whitelist", whitelistHandler.Create)
			r.Delete("/system/egress-whitelist/{id}", whitelistHandler.Delete)

			// Agent egress mode + blacklist (FEAT-016)
			r.Get("/system/egress/mode", egressHandler.GetMode)
			r.Put("/system/egress/mode", egressHandler.SetMode)
			r.Get("/system/egress-blacklist", egressHandler.ListBlacklist)
			r.Post("/system/egress-blacklist", egressHandler.CreateBlacklist)
			r.Delete("/system/egress-blacklist/{id}", egressHandler.DeleteBlacklist)

			// Agent sandbox default resource limit
			r.Get("/system/resource-limits", resourceLimitHandler.Get)
			r.Put("/system/resource-limits", resourceLimitHandler.Update)

			// Detached terminal session idle timeout (FEAT-022)
			r.Get("/system/session-idle-timeout", idleTimeoutHandler.Get)
			r.Put("/system/session-idle-timeout", idleTimeoutHandler.Update)

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
