package main

import "testing"

func TestIsAllowed(t *testing.T) {
	p := &proxyHandler{allowed: parseDomains("api.anthropic.com, api.openai.com ,Generativelanguage.Googleapis.com")}

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

func TestParseDomains(t *testing.T) {
	domains := parseDomains(" Foo.com , , bar.com ,bar.com")
	if len(domains) != 2 {
		t.Fatalf("expected 2 unique domains, got %d: %v", len(domains), domains)
	}
	if !domains["foo.com"] || !domains["bar.com"] {
		t.Errorf("expected foo.com and bar.com in %v", domains)
	}
}
