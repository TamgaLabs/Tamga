package git

import (
	"errors"
	"net/http"
	"time"

	"github.com/TamgaLabs/Tamga/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	queries *database.Queries
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{queries: database.New(pool)}
}

type CreateGitRepoRequest struct {
	URL    string `json:"url" binding:"required"`
	Branch string `json:"branch"`
}

type GitRepoResponse struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	URL       string    `json:"url"`
	Branch    string    `json:"branch"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toGitRepo(g database.GitRepository) GitRepoResponse {
	return GitRepoResponse{
		ID:        g.ID.String(),
		ProjectID: g.ProjectID.String(),
		URL:       g.Url,
		Branch:    g.Branch,
		CreatedAt: g.CreatedAt.Time,
		UpdatedAt: g.UpdatedAt.Time,
	}
}

func userID(c *gin.Context) pgtype.UUID {
	var uid pgtype.UUID
	uid.Scan(c.MustGet("user_id").(string))
	return uid
}

func projectID(c *gin.Context) pgtype.UUID {
	var pid pgtype.UUID
	pid.Scan(c.Param("projectId"))
	return pid
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateGitRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	branch := req.Branch
	if branch == "" {
		branch = "main"
	}

	repo, err := h.queries.CreateGitRepository(c.Request.Context(), database.CreateGitRepositoryParams{
		ProjectID: projectID(c),
		Url:       req.URL,
		Branch:    branch,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect git repository"})
		return
	}

	c.JSON(http.StatusCreated, toGitRepo(repo))
}

func (h *Handler) Get(c *gin.Context) {
	repo, err := h.queries.GetGitRepositoryByProject(c.Request.Context(), database.GetGitRepositoryByProjectParams{
		ProjectID: projectID(c),
		UserID:    userID(c),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "git repository not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch git repository"})
		return
	}

	c.JSON(http.StatusOK, toGitRepo(repo))
}

func (h *Handler) Delete(c *gin.Context) {
	var id pgtype.UUID
	if err := id.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid git repository id"})
		return
	}

	_, err := h.queries.DeleteGitRepository(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "git repository not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect git repository"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
