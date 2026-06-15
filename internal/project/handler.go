package project

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

type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type UpdateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type ProjectResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toProject(p database.Project) ProjectResponse {
	return ProjectResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		UserID:      p.UserID,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

func userID(c *gin.Context) string {
	return c.MustGet("user_id").(string)
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := h.queries.CreateProject(c.Request.Context(), database.CreateProjectParams{
		Name:        req.Name,
		Description: req.Description,
		UserID:      userID(c),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create project"})
		return
	}

	c.JSON(http.StatusCreated, toProject(project))
}

func (h *Handler) List(c *gin.Context) {
	projects, err := h.queries.ListProjectsByUser(c.Request.Context(), userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list projects"})
		return
	}

	resp := make([]ProjectResponse, len(projects))
	for i, p := range projects {
		resp[i] = toProject(p)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Get(c *gin.Context) {
	id := c.Param("projectId")

	project, err := h.queries.GetProjectByID(c.Request.Context(), database.GetProjectByIDParams{
		ID:     id,
		UserID: userID(c),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch project"})
		return
	}

	c.JSON(http.StatusOK, toProject(project))
}

func (h *Handler) Update(c *gin.Context) {
	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("projectId")

	project, err := h.queries.UpdateProject(c.Request.Context(), database.UpdateProjectParams{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		UserID:      userID(c),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update project"})
		return
	}

	c.JSON(http.StatusOK, toProject(project))
}

func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("projectId")

	_, err := h.queries.DeleteProject(c.Request.Context(), database.DeleteProjectParams{
		ID:     id,
		UserID: userID(c),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete project"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
