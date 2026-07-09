package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/gorilla/websocket"
)

// maxSessionsPerProject caps how many concurrent terminal sessions a single
// project's sandbox may have open at once.
const maxSessionsPerProject = 10

var (
	// ErrSessionCapExceeded is returned by CreateSession when a project
	// already has maxSessionsPerProject live sessions.
	ErrSessionCapExceeded = errors.New("project already has the maximum of 10 concurrent terminal sessions")
	// ErrSessionNotFound is returned when a session id doesn't exist (or
	// no longer does) for the given project.
	ErrSessionNotFound = errors.New("terminal session not found")
	// ErrSessionAlreadyAttached is returned by Attach when another
	// WebSocket connection is already live on the session.
	ErrSessionAlreadyAttached = errors.New("terminal session already has an active connection")
	// ErrSessionEnded is returned by Attach when the session's shell
	// process has already exited (e.g. it raced a terminate/natural exit
	// between lookup and attach).
	ErrSessionEnded = errors.New("terminal session has ended")
)

// TerminalSession is one persistent bash process running inside a
// project's sandbox container, identified by a short random id. Unlike the
// old connection-counted model (one WS = one anonymous exec), a session
// survives its owning WebSocket disconnecting: the shell process and its
// output-pumping goroutine (run, below) keep running server-side, feeding a
// scrollback ring buffer, until the session is explicitly terminated or the
// shell process exits on its own (e.g. the user typed `exit`).
type TerminalSession struct {
	ID            string
	ProjectID     int64
	ContainerName string
	CreatedAt     time.Time

	execID   string
	hijacked types.HijackedResponse
	ring     *ringBuffer

	// done is closed exactly once, by run's own cleanup, once the shell
	// process has exited and registry/sandbox cleanup has completed.
	done chan struct{}

	// wsMu guards every read/write of ws (the currently attached
	// connection, or nil) and serializes all writes onto it - both the
	// live output relay (from run) and the handler's own ping keepalive
	// go through this same lock so they never race as concurrent
	// gorilla/websocket writers.
	wsMu  sync.Mutex
	ws    *websocket.Conn
	ended bool
}

func newSessionID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Attach binds conn as this session's live output target, replaying any
// buffered scrollback to it first (under the same lock, so no live output
// can be interleaved before the replay finishes). Fails if another
// connection is already attached, or the session has already ended.
//
// Lock Ordering Invariant (FEAT-015 fix 2026-07-09): To prevent duplicate
// delivery of chunks (same chunk in replay snapshot + live relay), Attach's
// snapshot+subscribe must be atomic relative to relay's ring write + ws
// forward. This is achieved by: relay() holds wsMu across BOTH ring.Write
// and the ws.WriteMessage (see relay() comment), so by the time this
// function acquires wsMu, relay's mutation is complete. No chunk arrives
// between our Snapshot and our s.ws = conn assignment.
func (s *TerminalSession) Attach(conn *websocket.Conn) error {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()

	if s.ended {
		return ErrSessionEnded
	}
	if s.ws != nil {
		return ErrSessionAlreadyAttached
	}
	if backlog := s.ring.Snapshot(); len(backlog) > 0 {
		if err := conn.WriteMessage(websocket.BinaryMessage, backlog); err != nil {
			return err
		}
	}
	s.ws = conn
	return nil
}

// Detach releases conn as this session's live output target, if it is
// still the currently attached one (a stale Detach from an already
// superseded/ended attachment is a no-op).
func (s *TerminalSession) Detach(conn *websocket.Conn) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	if s.ws == conn {
		s.ws = nil
	}
}

// Ping sends a WebSocket ping control frame on the attached connection, if
// any. No-op (not an error) when detached.
func (s *TerminalSession) Ping() error {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	if s.ws == nil {
		return nil
	}
	return s.ws.WriteMessage(websocket.PingMessage, nil)
}

// WriteInput writes browser keystroke/paste data to the shell's PTY stdin.
func (s *TerminalSession) WriteInput(data []byte) error {
	_, err := s.hijacked.Conn.Write(data)
	return err
}

// Connected reports whether a WebSocket is currently attached to the
// session.
func (s *TerminalSession) Connected() bool {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	return s.ws != nil
}

// relay records a chunk of shell output into the scrollback ring buffer
// and, if a connection is currently attached, forwards it live. A failed
// write just drops the attachment (the browser's own read loop will notice
// the closed connection and detach cleanly) - it never kills the shell
// process.
//
// Lock Ordering (FEAT-015 fix 2026-07-09): Hold wsMu across BOTH the
// ring.Write AND the subsequent s.ws check/forward, making them atomic.
// This prevents a race where a snapshot+subscribe (Attach) interleaves
// between ring write and ws forward, causing the same chunk to be delivered
// twice (once via replay, once live). See Attach() comment for full trace.
func (s *TerminalSession) relay(p []byte) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()

	// Ring and ws forwarding must be atomic relative to Attach's snapshot+subscribe
	s.ring.Write(p)
	if s.ws != nil {
		if err := s.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
			s.ws = nil
		}
	}
}

// run pumps the shell's output into the session for as long as the shell
// process is alive, then performs the session's one-time end-of-life
// cleanup: closing the exec's stdio stream, closing any still-attached
// WebSocket, deregistering the session, and (via endSession) stopping the
// sandbox if this was the project's last session. It is started exactly
// once per session, right after creation, independent of any particular
// WebSocket ever attaching.
func (s *TerminalSession) run(agentSvc *AgentService) {
	defer func() {
		s.hijacked.Close()

		s.wsMu.Lock()
		s.ended = true
		if s.ws != nil {
			s.ws.Close()
			s.ws = nil
		}
		s.wsMu.Unlock()

		agentSvc.endSession(s)
		close(s.done)
	}()

	buf := make([]byte, 4096)
	for {
		n, err := s.hijacked.Reader.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			s.relay(chunk)
		}
		if err != nil {
			return
		}
	}
}

// SessionInfo is the REST-facing view of a session (see AgentService.ListSessions).
type SessionInfo struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Connected bool      `json:"connected"`
}

// sessionRegistry owns every project's live TerminalSessions plus a
// per-project mutex used to serialize the slow sandbox lifecycle
// operations (ensure-running, stop) for that project only - so one
// project's teardown never blocks another project's session create/attach.
// The registry's own map access (add/remove/get/count/list) is guarded by
// a separate, always-fast mutex, so a caller holding one project's business
// lock can still cheaply read another project's session count (see
// activeNetworks).
type sessionRegistry struct {
	lockMu sync.Mutex
	locks  map[int64]*sync.Mutex

	mu        sync.Mutex
	byProject map[int64]map[string]*TerminalSession
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{
		locks:     make(map[int64]*sync.Mutex),
		byProject: make(map[int64]map[string]*TerminalSession),
	}
}

// projectLock returns the mutex that serializes session create/terminate
// (and the sandbox ensure/stop decisions that go with them) for one
// project. Distinct projects get distinct mutexes so they never block each
// other.
func (r *sessionRegistry) projectLock(projectID int64) *sync.Mutex {
	r.lockMu.Lock()
	defer r.lockMu.Unlock()
	l, ok := r.locks[projectID]
	if !ok {
		l = &sync.Mutex{}
		r.locks[projectID] = l
	}
	return l
}

func (r *sessionRegistry) add(projectID int64, sess *TerminalSession) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byProject[projectID]
	if !ok {
		m = make(map[string]*TerminalSession)
		r.byProject[projectID] = m
	}
	m[sess.ID] = sess
}

func (r *sessionRegistry) remove(projectID int64, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byProject[projectID]
	if !ok {
		return
	}
	delete(m, sessionID)
	if len(m) == 0 {
		delete(r.byProject, projectID)
	}
}

func (r *sessionRegistry) get(projectID int64, sessionID string) (*TerminalSession, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.byProject[projectID]
	if !ok {
		return nil, false
	}
	s, ok := m[sessionID]
	return s, ok
}

func (r *sessionRegistry) count(projectID int64) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byProject[projectID])
}

func (r *sessionRegistry) list(projectID int64) []*TerminalSession {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.byProject[projectID]
	out := make([]*TerminalSession, 0, len(m))
	for _, s := range m {
		out = append(out, s)
	}
	return out
}

// activeNetworks returns the sandbox networks of every project that
// currently has at least one live session - used to keep the shared
// egress proxy attached to every network it needs to be on.
func (r *sessionRegistry) activeNetworks() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var nets []string
	for projectID, m := range r.byProject {
		if len(m) > 0 {
			nets = append(nets, agentNetworkName(projectID))
		}
	}
	return nets
}
