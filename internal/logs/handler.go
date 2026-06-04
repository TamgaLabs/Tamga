package logs

import (
	"errors"
	"net/http"

	"github.com/TamgaLabs/Tamga/internal/database"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	queries  *database.Queries
	streamer *Streamer
}

func NewHandler(pool *pgxpool.Pool, streamer *Streamer) *Handler {
	return &Handler{
		queries:  database.New(pool),
		streamer: streamer,
	}
}

func (h *Handler) StreamLogs(c *gin.Context) {
	var id pgtype.UUID
	if err := id.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	rawID, _ := c.Get("user_id")
	var uid pgtype.UUID
	uid.Scan(rawID.(string))

	// Verify deployment exists and belongs to user
	_, err := h.queries.GetDeploymentByID(c.Request.Context(), database.GetDeploymentByIDParams{
		ID:     id,
		UserID: uid,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch deployment"})
		return
	}

	conn, err := Upgrade(c.Writer, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade connection"})
		return
	}

	h.streamer.StreamDeploymentLogs(c.Request.Context(), id, conn)
}
