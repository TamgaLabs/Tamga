package main

import "testing"

func TestIsAllowedWhitelistMode(t *testing.T) {
	p := &proxyHandler{
		mode:    "whitelist",
		allowed: parseDomains("api.anthropic.com, api.openai.com ,Generativelanguage.Googleapis.com"),
	}

	cases := []struct {
		hostport string
		want     bool
	}{
		{"api.anthropic.com:443", true},
		{"api.anthropic.com", true},
		{"generativelanguage.googleapis.com:443", true}, // case-insensitive
		{"evil.example.com:443", false},
		{"api.openai.com.evil.com:443", false}, // no suffix/substring matching
		{"api.openai.com.:443", true},          // trailing dot normalized
	}

	for _, c := range cases {
		if got := p.isAllowed(c.hostport); got != c.want {
			t.Errorf("isAllowed(%q) = %v, want %v", c.hostport, got, c.want)
		}
	}
}

func TestIsAllowedBlacklistMode(t *testing.T) {
	p := &proxyHandler{
		mode:   "blacklist",
		denied: parseDomains("evil.example.com, Tracker.Example.com"),
	}

	cases := []struct {
		hostport string
		want     bool
	}{
		{"evil.example.com:443", false},
		{"tracker.example.com:443", false}, // case-insensitive
		{"api.anthropic.com:443", true},    // not on the blacklist - allowed
		{"evil.example.com.:443", false},   // trailing dot normalized
	}

	for _, c := range cases {
		if got := p.isAllowed(c.hostport); got != c.want {
			t.Errorf("isAllowed(%q) = %v, want %v", c.hostport, got, c.want)
		}
	}
}

func TestIsAllowedOpenMode(t *testing.T) {
	// Explicit "open" mode and an unset/unknown mode both fall through to
	// allow-everything - the deliberate default behavior (see FEAT-016).
	for _, mode := range []string{"open", "", "bogus"} {
		p := &proxyHandler{mode: mode}
		if !p.isAllowed("anything.example.com:443") {
			t.Errorf("isAllowed with mode %q = false, want true", mode)
		}
	}
}

func TestParseDomains(t *testing.T) {
	domains := parseDomains(" Foo.com , , bar.com ,bar.com")
	if len(domains) != 2 {
		t.Fatalf("expected 2 unique domains, got %d: %v", len(domains), domains)
	}
	if !domains["foo.com"] || !domains["bar.com"] {
		t.Errorf("expected foo.com and bar.com in %v", domains)
	}
}
