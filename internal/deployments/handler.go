package deployments

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
	service *Service
}

func NewHandler(pool *pgxpool.Pool, service *Service) *Handler {
	return &Handler{
		queries: database.New(pool),
		service: service,
	}
}

type DeploymentResponse struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Status        string    `json:"status"`
	CommitSHA     string    `json:"commit_sha"`
	CommitMessage string    `json:"commit_message"`
	ImageTag      string    `json:"image_tag"`
	ContainerID   string    `json:"container_id"`
	Domain        string    `json:"domain"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DeploymentLogResponse struct {
	ID           string    `json:"id"`
	DeploymentID string    `json:"deployment_id"`
	Stream       string    `json:"stream"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}

func toDeployment(d database.Deployment) DeploymentResponse {
	return DeploymentResponse{
		ID:            d.ID.String(),
		ProjectID:     d.ProjectID.String(),
		Status:        d.Status,
		CommitSHA:     d.CommitSha,
		CommitMessage: d.CommitMessage,
		ImageTag:      d.ImageTag,
		ContainerID:   d.ContainerID,
		Domain:        d.Domain,
		CreatedAt:     d.CreatedAt.Time,
		UpdatedAt:     d.UpdatedAt.Time,
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
	uid := userID(c)
	pid := projectID(c)

	deployment, err := h.queries.CreateDeployment(c.Request.Context(), pid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create deployment"})
		return
	}

	go h.service.Deploy(c.Request.Context(), DeployParams{
		DeploymentID: deployment.ID,
		ProjectID:    pid,
		UserID:       uid,
	})

	c.JSON(http.StatusAccepted, toDeployment(deployment))
}

func (h *Handler) List(c *gin.Context) {
	deployments, err := h.queries.ListDeploymentsByProject(c.Request.Context(), database.ListDeploymentsByProjectParams{
		ProjectID: projectID(c),
		UserID:    userID(c),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list deployments"})
		return
	}

	resp := make([]DeploymentResponse, len(deployments))
	for i, d := range deployments {
		resp[i] = toDeployment(d)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Get(c *gin.Context) {
	var id pgtype.UUID
	if err := id.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	deployment, err := h.queries.GetDeploymentByID(c.Request.Context(), database.GetDeploymentByIDParams{
		ID:     id,
		UserID: userID(c),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch deployment"})
		return
	}

	c.JSON(http.StatusOK, toDeployment(deployment))
}

func (h *Handler) Restart(c *gin.Context) {
	var id pgtype.UUID
	if err := id.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	uid := userID(c)

	deployment, err := h.queries.GetDeploymentByID(c.Request.Context(), database.GetDeploymentByIDParams{
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

	go h.service.Deploy(c.Request.Context(), DeployParams{
		DeploymentID: deployment.ID,
		ProjectID:    deployment.ProjectID,
		UserID:       uid,
	})

	c.JSON(http.StatusAccepted, gin.H{"message": "redeploy started"})
}

func (h *Handler) Logs(c *gin.Context) {
	var id pgtype.UUID
	if err := id.Scan(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deployment id"})
		return
	}

	_, err := h.queries.GetDeploymentByID(c.Request.Context(), database.GetDeploymentByIDParams{
		ID:     id,
		UserID: userID(c),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch deployment"})
		return
	}

	logs, err := h.queries.ListDeploymentLogs(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch logs"})
		return
	}

	resp := make([]DeploymentLogResponse, len(logs))
	for i, l := range logs {
		resp[i] = DeploymentLogResponse{
			ID:           l.ID.String(),
			DeploymentID: l.DeploymentID.String(),
			Stream:       l.Stream,
			Message:      l.Message,
			CreatedAt:    l.CreatedAt.Time,
		}
	}
	c.JSON(http.StatusOK, resp)
}
