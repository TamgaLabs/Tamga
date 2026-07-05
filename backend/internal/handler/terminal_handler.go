package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// Keepalive tuning for the terminal WebSocket: if a client disconnects
// without sending a close frame (laptop sleep, network drop), the periodic
// ping lets us notice within pongWait and tear the connection (and its
// sandbox refcount) down instead of leaning on OS-level TCP keepalive.
const (
	pongWait   = 60 * time.Second
	pingPeriod = pongWait * 9 / 10
)

// terminalUpgrader upgrades the terminal WebSocket connection. Origin
// checking is skipped because the request is already authenticated by
// AuthMiddleware (via the Authorization header or a `token` query param,
// since the browser WebSocket API cannot set custom headers).
var terminalUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// terminalClientMessage is what the browser sends over the WebSocket: either
// a chunk of keystroke input or a resize event. Server -> client messages are
// raw binary shell output, with no envelope, since that direction never
// needs anything but bytes.
type terminalClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint   `json:"cols,omitempty"`
	Rows uint   `json:"rows,omitempty"`
}

type TerminalHandler struct {
	agentSvc *service.AgentService
}

func NewTerminalHandler(agentSvc *service.AgentService) *TerminalHandler {
	return &TerminalHandler{agentSvc: agentSvc}
}

// Serve creates the project's sandbox container on demand, starts a shell
// inside it and proxies stdin/stdout/resize over a WebSocket for the
// lifetime of the connection. When the connection closes (tab close or
// explicit disconnect), the sandbox container is stopped and removed.
func (h *TerminalHandler) Serve(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	containerName, workDir, err := h.agentSvc.StartSandbox(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	conn, err := terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("terminal websocket upgrade failed", "error", err)
		h.agentSvc.ReleaseSandbox(context.Background(), projectID)
		return
	}
	defer conn.Close()
	defer h.agentSvc.ReleaseSandbox(context.Background(), projectID)

	// Ping/pong keepalive: without this, an ungraceful disconnect (no close
	// frame) can go unnoticed by ReadMessage for a long time, leaving the
	// sandbox container running with a phantom refcount.
	var writeMu sync.Mutex
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				writeMu.Lock()
				err := conn.WriteMessage(websocket.PingMessage, nil)
				writeMu.Unlock()
				if err != nil {
					return
				}
			case <-stopPing:
				return
			}
		}
	}()

	execID, err := h.agentSvc.OpenShell(r.Context(), containerName, workDir)
	if err != nil {
		slog.Error("open shell failed", "container", containerName, "error", err)
		writeMu.Lock()
		conn.WriteMessage(websocket.TextMessage, []byte("failed to start shell: "+err.Error()))
		writeMu.Unlock()
		return
	}

	hijacked, err := h.agentSvc.AttachShell(r.Context(), execID)
	if err != nil {
		slog.Error("attach shell failed", "container", containerName, "error", err)
		return
	}
	defer hijacked.Close()

	// Shell output -> browser. Closing readerDone signals the shell process
	// (and therefore the connection) is done, so we force the read loop
	// below to unblock too.
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		buf := make([]byte, 4096)
		for {
			n, err := hijacked.Reader.Read(buf)
			if n > 0 {
				writeMu.Lock()
				werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				writeMu.Unlock()
				if werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		<-readerDone
		conn.Close()
	}()

	// Browser -> shell input/resize.
readLoop:
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if msgType != websocket.TextMessage {
			continue
		}
		var msg terminalClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "input":
			if _, werr := hijacked.Conn.Write([]byte(msg.Data)); werr != nil {
				break readLoop
			}
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				if rerr := h.agentSvc.ResizeShell(r.Context(), execID, msg.Rows, msg.Cols); rerr != nil {
					slog.Warn("resize shell failed", "container", containerName, "error", rerr)
				}
			}
		}
	}

	hijacked.Close()
	<-readerDone
}
