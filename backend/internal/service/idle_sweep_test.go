package service

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// This file is a deliberate exception to FEAT-021's move of tests into
// internal/tests/: idleSessions and TerminalSession's ws/lastDetachAt/ended
// fields are unexported, and these tests build TerminalSession values with
// those fields set directly (idleTestSession, below) to control
// attached/detached state and idle duration without a real WebSocket or
// Docker daemon - deliberately, so the idle-timeout sweep's selection logic
// (FEAT-022) can be unit tested in isolation. There is no exported surface
// to drive this through, so it stays colocated (same precedent as
// terminal_session_registry_test.go and agent_service_test.go).

// idleTestSession builds a TerminalSession with just enough fields set to
// exercise IdleSince/idleSessions - no Docker exec/hijacked stream involved.
func idleTestSession(id string, ws *websocket.Conn, lastDetachAt time.Time, ended bool) *TerminalSession {
	return &TerminalSession{
		ID:           id,
		ring:         newRingBuffer(1024),
		done:         make(chan struct{}),
		ws:           ws,
		lastDetachAt: lastDetachAt,
		ended:        ended,
	}
}

func TestIdleSessionsNeverTimeoutSelectsNothing(t *testing.T) {
	now := time.Now()
	sessions := []*TerminalSession{
		idleTestSession("s1", nil, now.Add(-24*time.Hour), false),
	}
	if got := idleSessions(sessions, 0, now); len(got) != 0 {
		t.Fatalf("expected no sessions selected with a 0 (Never) timeout, got %d", len(got))
	}
}

// TestIdleSessionsSelectsOnlyDetachedPastTimeout is the core acceptance
// scenario: a long-detached session is selected, a recently-detached one
// isn't, an attached session isn't regardless of how long ago it was
// created/last detached, and an already-ended session isn't either.
func TestIdleSessionsSelectsOnlyDetachedPastTimeout(t *testing.T) {
	now := time.Now()
	timeout := 5 * time.Minute

	stillFresh := idleTestSession("fresh", nil, now.Add(-1*time.Minute), false)
	longIdle := idleTestSession("idle", nil, now.Add(-10*time.Minute), false)
	attachedOld := idleTestSession("attached", &websocket.Conn{}, now.Add(-1*time.Hour), false)
	ended := idleTestSession("ended", nil, now.Add(-1*time.Hour), true)

	got := idleSessions([]*TerminalSession{stillFresh, longIdle, attachedOld, ended}, timeout, now)

	if len(got) != 1 || got[0] != longIdle {
		t.Fatalf("expected exactly the long-idle detached session selected, got %v", got)
	}
}

func TestIdleSessionsExactBoundaryIsSelected(t *testing.T) {
	now := time.Now()
	timeout := 5 * time.Minute
	sess := idleTestSession("boundary", nil, now.Add(-timeout), false)

	if got := idleSessions([]*TerminalSession{sess}, timeout, now); len(got) != 1 {
		t.Fatalf("expected a session idle for exactly the timeout to be selected (>= comparison), got %d", len(got))
	}
}

// TestIdleSessionsAcrossRegistry verifies idleSessions works end to end
// against sessionRegistry.all() - the same input sweepIdleSessions feeds it
// in production - across multiple projects at once.
func TestIdleSessionsAcrossRegistry(t *testing.T) {
	r := newSessionRegistry()
	now := time.Now()
	timeout := time.Minute

	r.add(1, idleTestSession("a", nil, now.Add(-2*time.Minute), false))
	r.add(2, idleTestSession("b", nil, now.Add(-2*time.Minute), false))
	r.add(2, idleTestSession("c", &websocket.Conn{}, now.Add(-2*time.Minute), false))

	got := idleSessions(r.all(), timeout, now)
	if len(got) != 2 {
		t.Fatalf("expected 2 idle sessions selected across both projects, got %d", len(got))
	}
}

// TestTerminalSessionDetachUpdatesLastDetachAt exercises the real
// Attach/Detach/IdleSince interplay (not the fake-field shortcut the other
// tests use), confirming a session is never idle while attached and starts
// its idle clock the moment it's detached.
func TestTerminalSessionDetachUpdatesLastDetachAt(t *testing.T) {
	sess := idleTestSession("s", nil, time.Time{}, false)
	conn := &websocket.Conn{}

	if err := sess.Attach(conn); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if _, ok := sess.IdleSince(); ok {
		t.Fatal("expected an attached session to never be idle")
	}

	sess.Detach(conn)

	idleSince, ok := sess.IdleSince()
	if !ok {
		t.Fatal("expected a detached session to be idle")
	}
	if time.Since(idleSince) > time.Second {
		t.Fatalf("expected lastDetachAt to be set to ~now on detach, got %v", idleSince)
	}
}
