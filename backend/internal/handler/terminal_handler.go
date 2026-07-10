package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// unattachedSessionCleanupTimeout bounds how long Serve's deferred cleanup
// (see BUG-027) will wait for a just-created-but-never-attached session to
// terminate. It uses its own context rather than the request's, since the
// request's context is commonly already canceled in exactly the scenario
// this cleanup exists for (aborted handshake / failed upgrade).
const unattachedSessionCleanupTimeout = 5 * time.Second

// Keepalive tuning for the terminal WebSocket: if a client disconnects
// without sending a close frame (laptop sleep, network drop), the periodic
// ping lets us notice within pongWait and detach the session (not stop
// it - see FEAT-015) instead of leaning on OS-level TCP keepalive.
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

// Serve attaches a WebSocket connection to a project's terminal session,
// proxying stdin/stdout/resize over it for the lifetime of the connection.
//
// With no `?session=` query param, a brand new session is created (this is
// what the current frontend always does - see FEAT-015's frontend
// compatibility requirement). With `?session=<id>`, it reattaches to an
// existing, still-live session instead, replaying its scrollback first.
//
// Unlike the pre-FEAT-015 behavior, closing this WebSocket does not stop
// the shell process or the sandbox: the session keeps running server-side
// (see AgentService.CreateSession/TerminalSession.run) until it is
// explicitly terminated via DELETE .../agent/sessions/{sessionId} (see
// TerminateSession below) or the shell process exits on its own.
func (h *TerminalHandler) Serve(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	sessionID := r.URL.Query().Get("session")

	// attached flips true only once a newly-created session's WebSocket
	// has successfully Attach()'d (see below). It's declared here, at
	// function scope, so both the create branch's deferred cleanup and the
	// later Attach call can see it.
	attached := false

	var sess *service.TerminalSession
	if sessionID != "" {
		var ok bool
		sess, ok = h.agentSvc.GetSession(projectID, sessionID)
		if !ok {
			http.Error(w, "terminal session not found", http.StatusNotFound)
			return
		}
	} else {
		sess, err = h.agentSvc.CreateSession(r.Context(), projectID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, service.ErrSessionCapExceeded) {
				status = http.StatusTooManyRequests
			}
			http.Error(w, err.Error(), status)
			return
		}

		// BUG-027: a session created above only becomes useful once a
		// WebSocket successfully attaches to it below. If the upgrade
		// fails, the client aborts the handshake, or Attach itself fails,
		// this newly-created session must not be left registered - it
		// would orphan (invisible to any client, eating a cap slot,
		// keeping the sandbox alive) forever. Until attached flips true,
		// this deferred cleanup terminates the session on any early
		// return. A reattach (sessionID != "") never runs this - a failed
		// reattach must not tear down an existing, possibly-in-use
		// session.
		defer func() {
			if attached {
				return
			}
			cleanupCtx, cancel := context.WithTimeout(context.Background(), unattachedSessionCleanupTimeout)
			defer cancel()
			if terr := h.agentSvc.TerminateSession(cleanupCtx, projectID, sess.ID); terr != nil {
				slog.Warn("failed to clean up unattached terminal session", "session_id", sess.ID, "project_id", projectID, "error", terr)
			}
		}()
	}

	conn, err := terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("terminal websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	if err := sess.Attach(conn); err != nil {
		slog.Warn("terminal session attach failed", "session_id", sess.ID, "project_id", projectID, "error", err)
		conn.WriteMessage(websocket.TextMessage, []byte(err.Error()))
		return
	}
	attached = true
	defer sess.Detach(conn)

	// Ping/pong keepalive: without this, an ungraceful disconnect (no close
	// frame) can go unnoticed by ReadMessage for a long time, leaving the
	// session attached to a dead connection.
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
				if err := sess.Ping(); err != nil {
					return
				}
			case <-stopPing:
				return
			}
		}
	}()

	// Browser -> shell input/resize. Shell -> browser output is pumped by
	// the session's own long-lived goroutine (started once at session
	// creation, independent of this connection's lifetime) via sess.Attach
	// above; if the shell process exits while we're attached, the session
	// closes this conn itself, which unblocks ReadMessage below.
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
			if werr := sess.WriteInput([]byte(msg.Data)); werr != nil {
				return
			}
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				if rerr := h.agentSvc.ResizeShell(r.Context(), sess, msg.Rows, msg.Cols); rerr != nil {
					slog.Warn("resize shell failed", "session_id", sess.ID, "error", rerr)
				}
			}
		}
	}
}

// ListSessions returns every live terminal session for a project.
func (h *TerminalHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(h.agentSvc.ListSessions(projectID))
}

// TerminateSession explicitly ends one session (kills its shell process);
// if it was the project's last session, the sandbox is stopped too.
func (h *TerminalHandler) TerminateSession(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	sessionID := chi.URLParam(r, "sessionId")

	if err := h.agentSvc.TerminateSession(r.Context(), projectID, sessionID); err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
