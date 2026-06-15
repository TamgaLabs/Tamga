package logs

import (
	"bufio"
	"context"
	"net/http"
	"time"

	"github.com/TamgaLabs/Tamga/internal/database"
	dockerclient "github.com/TamgaLabs/Tamga/internal/docker"
	"github.com/gorilla/websocket"
)

type LogMessage struct {
	Type      string    `json:"type"`
	Stream    string    `json:"stream,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type Streamer struct {
	queries *database.Queries
	docker  *dockerclient.Client
}

func NewStreamer(queries *database.Queries, docker *dockerclient.Client) *Streamer {
	return &Streamer{queries: queries, docker: docker}
}

func (s *Streamer) StreamDeploymentLogs(ctx context.Context, deploymentID string, conn *websocket.Conn) {
	defer conn.Close()

	logs, err := s.queries.ListDeploymentLogs(ctx, deploymentID)
	if err == nil {
		for _, l := range logs {
			msg := LogMessage{
				Type:      "log",
				Stream:    l.Stream,
				Message:   l.Message,
				Timestamp: l.CreatedAt,
			}
			if err := writeJSON(conn, msg); err != nil {
				return
			}
		}
	}

	deployment, err := s.queries.GetDeploymentByIDNoAuth(ctx, deploymentID)
	if err != nil || deployment.ContainerID == "" {
		writeJSON(conn, LogMessage{
			Type:      "status",
			Message:   "deployment has no running container",
			Timestamp: time.Now(),
		})
		return
	}

	logReader, err := s.docker.ContainerLogs(ctx, deployment.ContainerID, "all", true)
	if err != nil {
		writeJSON(conn, LogMessage{
			Type:      "error",
			Message:   "failed to attach to container logs",
			Timestamp: time.Now(),
		})
		return
	}
	defer logReader.Close()

	scanner := bufio.NewScanner(logReader)
	for scanner.Scan() {
		line := scanner.Text()
		cleanLine := cleanDockerLogLine(line)

		msg := LogMessage{
			Type:      "log",
			Stream:    "stdout",
			Message:   cleanLine,
			Timestamp: time.Now(),
		}
		if err := writeJSON(conn, msg); err != nil {
			return
		}
	}

	writeJSON(conn, LogMessage{
		Type:      "status",
		Message:   "log stream ended",
		Timestamp: time.Now(),
	})
}

func writeJSON(conn *websocket.Conn, msg LogMessage) error {
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(msg)
}

func cleanDockerLogLine(line string) string {
	if len(line) > 8 {
		if line[0] == 0x01 || line[0] == 0x02 {
			return line[8:]
		}
	}
	return line
}

func Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	return upgrader.Upgrade(w, r, nil)
}
