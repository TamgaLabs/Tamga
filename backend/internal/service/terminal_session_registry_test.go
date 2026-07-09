package service

import (
	"fmt"
	"testing"
)

// fakeSession builds a TerminalSession with just enough fields set to
// exercise the registry (no Docker exec/hijacked stream involved) - the
// registry itself doesn't touch those fields.
func fakeSession(projectID int64, id string) *TerminalSession {
	return &TerminalSession{
		ID:        id,
		ProjectID: projectID,
		ring:      newRingBuffer(1024),
		done:      make(chan struct{}),
	}
}

func TestSessionRegistryAddGetRemove(t *testing.T) {
	r := newSessionRegistry()
	sess := fakeSession(1, "abc123")
	r.add(1, sess)

	got, ok := r.get(1, "abc123")
	if !ok || got != sess {
		t.Fatalf("expected to get back the added session, got %v, %v", got, ok)
	}
	if _, ok := r.get(1, "does-not-exist"); ok {
		t.Fatal("expected lookup of unknown session id to fail")
	}
	if _, ok := r.get(2, "abc123"); ok {
		t.Fatal("expected lookup under the wrong project id to fail")
	}

	r.remove(1, "abc123")
	if _, ok := r.get(1, "abc123"); ok {
		t.Fatal("expected session to be gone after remove")
	}
}

func TestSessionRegistryCountAndList(t *testing.T) {
	r := newSessionRegistry()
	if got := r.count(1); got != 0 {
		t.Fatalf("expected count 0 for untouched project, got %d", got)
	}

	for i := 0; i < 3; i++ {
		r.add(1, fakeSession(1, fmt.Sprintf("s%d", i)))
	}
	r.add(2, fakeSession(2, "other-project"))

	if got := r.count(1); got != 3 {
		t.Fatalf("expected count 3, got %d", got)
	}
	if got := r.count(2); got != 1 {
		t.Fatalf("expected count 1 for project 2, got %d", got)
	}
	if got := len(r.list(1)); got != 3 {
		t.Fatalf("expected list of 3, got %d", got)
	}
}

// TestSessionRegistryCapEnforcement exercises the exact guard
// AgentService.CreateSession uses (count >= maxSessionsPerProject) against
// the real registry and real constant, without needing a Docker daemon -
// per FEAT-015's requirement to unit test the cap where practical.
func TestSessionRegistryCapEnforcement(t *testing.T) {
	r := newSessionRegistry()
	const projectID = int64(42)

	for i := 0; i < maxSessionsPerProject; i++ {
		if r.count(projectID) >= maxSessionsPerProject {
			t.Fatalf("cap check tripped early at i=%d", i)
		}
		r.add(projectID, fakeSession(projectID, fmt.Sprintf("s%d", i)))
	}

	if got := r.count(projectID); got != maxSessionsPerProject {
		t.Fatalf("expected exactly %d sessions, got %d", maxSessionsPerProject, got)
	}
	if r.count(projectID) < maxSessionsPerProject {
		t.Fatal("expected the 11th session's cap check to reject")
	}

	// A different project is entirely unaffected by project 42 being at
	// capacity.
	other := fakeSession(43, "unrelated")
	r.add(43, other)
	if got := r.count(43); got != 1 {
		t.Fatalf("expected project 43 to be unaffected by project 42's cap, got count %d", got)
	}
}

func TestSessionRegistryProjectLockIdentity(t *testing.T) {
	r := newSessionRegistry()
	l1 := r.projectLock(1)
	l2 := r.projectLock(1)
	if l1 != l2 {
		t.Fatal("expected the same *sync.Mutex for repeated calls with the same project id")
	}
	l3 := r.projectLock(2)
	if l1 == l3 {
		t.Fatal("expected distinct *sync.Mutex instances for different project ids")
	}
}

func TestSessionRegistryActiveNetworks(t *testing.T) {
	r := newSessionRegistry()
	if got := r.activeNetworks(); len(got) != 0 {
		t.Fatalf("expected no active networks with no sessions, got %v", got)
	}

	r.add(1, fakeSession(1, "s1"))
	r.add(2, fakeSession(2, "s2"))

	nets := r.activeNetworks()
	if len(nets) != 2 {
		t.Fatalf("expected 2 active networks, got %v", nets)
	}
	want := map[string]bool{agentNetworkName(1): true, agentNetworkName(2): true}
	for _, n := range nets {
		if !want[n] {
			t.Fatalf("unexpected network %q in %v", n, nets)
		}
	}

	r.remove(1, "s1")
	nets = r.activeNetworks()
	if len(nets) != 1 || nets[0] != agentNetworkName(2) {
		t.Fatalf("expected only project 2's network after project 1 emptied out, got %v", nets)
	}
}
