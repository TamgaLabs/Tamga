package envvar

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/TamgaLabs/Tamga/internal/database"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	queries *database.Queries
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{queries: database.New(db)}
}

type CreateEnvVarRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

type UpdateEnvVarRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value"`
}

type EnvVarResponse struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toEnvVar(e database.EnvVar) EnvVarResponse {
	return EnvVarResponse{
		ID:        e.ID,
		ProjectID: e.ProjectID,
		Key:       e.Key,
		Value:     e.Value,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

func userID(c *gin.Context) string {
	return c.MustGet("user_id").(string)
}

func projectID(c *gin.Context) string {
	return c.Param("projectId")
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateEnvVarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	envVar, err := h.queries.CreateEnvVar(c.Request.Context(), database.CreateEnvVarParams{
		ProjectID: projectID(c),
		Key:       req.Key,
		Value:     req.Value,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create env var"})
		return
	}

	c.JSON(http.StatusCreated, toEnvVar(envVar))
}

func (h *Handler) List(c *gin.Context) {
	envVars, err := h.queries.ListEnvVarsByProject(c.Request.Context(), database.ListEnvVarsByProjectParams{
		ProjectID: projectID(c),
		UserID:    userID(c),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list env vars"})
		return
	}

	resp := make([]EnvVarResponse, len(envVars))
	for i, e := range envVars {
		resp[i] = toEnvVar(e)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateEnvVarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("id")

	envVar, err := h.queries.UpdateEnvVar(c.Request.Context(), database.UpdateEnvVarParams{
		ID:    id,
		Key:   req.Key,
		Value: req.Value,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "env var not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update env var"})
		return
	}

	c.JSON(http.StatusOK, toEnvVar(envVar))
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")

	_, err := h.queries.DeleteEnvVar(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "env var not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete env var"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
