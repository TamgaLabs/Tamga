package domain

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

type CreateDomainRequest struct {
	Domain string `json:"domain" binding:"required"`
}

type DomainResponse struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Domain    string    `json:"domain"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toDomain(d database.Domain) DomainResponse {
	return DomainResponse{
		ID:        d.ID.String(),
		ProjectID: d.ProjectID.String(),
		Domain:    d.Domain,
		Verified:  d.Verified,
		CreatedAt: d.CreatedAt.Time,
		UpdatedAt: d.UpdatedAt.Time,
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
	var req CreateDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	domain, err := h.queries.CreateDomain(c.Request.Context(), database.CreateDomainParams{
		ProjectID: projectID(c),
		Domain:    req.Domain,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create domain"})
		return
	}

	c.JSON(http.StatusCreated, toDomain(domain))
}

func (h *Handler) List(c *gin.Context) {
	domains, err := h.queries.ListDomainsByProject(c.Request.Context(), database.ListDomainsByProjectParams{
		ProjectID: projectID(c),
		UserID:    userID(c),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list domains"})
		return
	}

	resp := make([]DomainResponse, len(domains))
	for i, d := range domains {
		resp[i] = toDomain(d)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Delete(c *gin.Context) {
	var id pgtype.UUID
	if err := id.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
		return
	}

	_, err := h.queries.DeleteDomain(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "domain not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete domain"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
