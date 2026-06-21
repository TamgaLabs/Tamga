package api

import (
	"net/http"

	"github.com/TamgaLabs/Tamga/internal/feature/auth"
	"github.com/TamgaLabs/Tamga/internal/feature/project"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	Auth    *auth.Handler
	Project *project.Handler
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
