package logs

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/TamgaLabs/Tamga/internal/database"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	queries  *database.Queries
	streamer *Streamer
}

func NewHandler(db *sql.DB, streamer *Streamer) *Handler {
	return &Handler{
		queries:  database.New(db),
		streamer: streamer,
	}
}

func (h *Handler) StreamLogs(c *gin.Context) {
	id := c.Param("id")
	uid := c.MustGet("user_id").(string)

	_, err := h.queries.GetDeploymentByID(c.Request.Context(), database.GetDeploymentByIDParams{
		ID:     id,
		UserID: uid,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
