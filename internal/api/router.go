package api

import (
	"net/http"

	"github.com/TamgaLabs/Tamga/internal/auth"
	"github.com/TamgaLabs/Tamga/internal/deployments"
	"github.com/TamgaLabs/Tamga/internal/domain"
	"github.com/TamgaLabs/Tamga/internal/envvar"
	"github.com/TamgaLabs/Tamga/internal/git"
	"github.com/TamgaLabs/Tamga/internal/logs"
	"github.com/TamgaLabs/Tamga/internal/project"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	Auth       *auth.Handler
	Project    *project.Handler
	Domain     *domain.Handler
	EnvVar     *envvar.Handler
	Git        *git.Handler
	Deployment *deployments.Handler
	Logs       *logs.Handler
}

func SetupRouter(h *Handlers, jwtSecret string) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", h.Auth.Register)
			authGroup.POST("/login", h.Auth.Login)
		}

		protected := api.Group("")
		protected.Use(auth.Middleware(jwtSecret))
		{
			protected.GET("/auth/me", h.Auth.Me)

			protected.POST("/projects", h.Project.Create)
			protected.GET("/projects", h.Project.List)
			protected.GET("/projects/:projectId", h.Project.Get)
			protected.PUT("/projects/:projectId", h.Project.Update)
			protected.DELETE("/projects/:projectId", h.Project.Delete)

			protected.POST("/projects/:projectId/domains", h.Domain.Create)
			protected.GET("/projects/:projectId/domains", h.Domain.List)
			protected.DELETE("/domains/:id", h.Domain.Delete)

			protected.POST("/projects/:projectId/env-vars", h.EnvVar.Create)
			protected.GET("/projects/:projectId/env-vars", h.EnvVar.List)
			protected.PUT("/env-vars/:id", h.EnvVar.Update)
			protected.DELETE("/env-vars/:id", h.EnvVar.Delete)

			protected.POST("/projects/:projectId/git", h.Git.Create)
			protected.GET("/projects/:projectId/git", h.Git.Get)
			protected.DELETE("/git/:id", h.Git.Delete)

			protected.POST("/projects/:projectId/deployments", h.Deployment.Create)
			protected.GET("/projects/:projectId/deployments", h.Deployment.List)
			protected.GET("/deployments/:id", h.Deployment.Get)
			protected.POST("/deployments/:id/restart", h.Deployment.Restart)
			protected.GET("/deployments/:id/logs", h.Deployment.Logs)
			protected.GET("/deployments/:id/logs/stream", h.Logs.StreamLogs)
		}
	}

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
