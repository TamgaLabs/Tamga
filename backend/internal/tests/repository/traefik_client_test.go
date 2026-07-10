package repository_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/TamgaLabs/Tamga/backend/internal/repository/traefik"
)

// parsedDynamicConfig mirrors the shape traefik.Client.AddRoute writes
// (http.routers/http.services), just enough to unmarshal and assert
// against without needing access to the package's own unexported struct.
type parsedDynamicConfig struct {
	HTTP struct {
		Routers map[string]struct {
			Rule        string    `yaml:"rule"`
			Service     string    `yaml:"service"`
			EntryPoints []string  `yaml:"entryPoints"`
			TLS         *struct{} `yaml:"tls"`
		} `yaml:"routers"`
		Services map[string]struct {
			LoadBalancer struct {
				Servers []struct {
					URL string `yaml:"url"`
				} `yaml:"servers"`
			} `yaml:"loadBalancer"`
		} `yaml:"services"`
	} `yaml:"http"`
}

// TestTraefikClientAddRouteWritesSplitRouters covers FEAT-024's core
// requirement: the generated dynamic-config file for a project must
// contain a split plain-HTTP + TLS router pair (FEAT-023's
// Implementation Notes found a single dual-entrypoint `tls: {}` router
// silently doesn't serve plain HTTP), both pointing at one service named
// exactly "project-<id>" (not the domain) so Traefik's per-router/service
// Prometheus metrics stay attributable back to the project (TEST-010 §4).
func TestTraefikClientAddRouteWritesSplitRouters(t *testing.T) {
	dir := t.TempDir()
	client := traefik.New(dir)

	if err := client.AddRoute(42, "myapp.example.com", "project-42:3000"); err != nil {
		t.Fatalf("AddRoute: %v", err)
	}

	path := filepath.Join(dir, "project-42.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}

	var cfg parsedDynamicConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("generated file is not valid YAML: %v\ncontent:\n%s", err, raw)
	}

	// Two routers: plain + "-secure", both service "project-42".
	plain, ok := cfg.HTTP.Routers["project-42"]
	if !ok {
		t.Fatalf("expected router %q, got routers: %+v", "project-42", cfg.HTTP.Routers)
	}
	if plain.Service != "project-42" {
		t.Errorf("plain router service = %q, want %q", plain.Service, "project-42")
	}
	if plain.Rule != "Host(`myapp.example.com`)" {
		t.Errorf("plain router rule = %q, want Host(`myapp.example.com`)", plain.Rule)
	}
	if len(plain.EntryPoints) != 1 || plain.EntryPoints[0] != "web" {
		t.Errorf("plain router entryPoints = %v, want [web]", plain.EntryPoints)
	}
	if plain.TLS != nil {
		t.Errorf("plain router has a tls key set; the plain router must have none (FEAT-023's split-router fix - a tls-having router doesn't serve plain HTTP)")
	}

	secure, ok := cfg.HTTP.Routers["project-42-secure"]
	if !ok {
		t.Fatalf("expected router %q, got routers: %+v", "project-42-secure", cfg.HTTP.Routers)
	}
	if secure.Service != "project-42" {
		t.Errorf("secure router service = %q, want %q", secure.Service, "project-42")
	}
	if secure.Rule != "Host(`myapp.example.com`)" {
		t.Errorf("secure router rule = %q, want Host(`myapp.example.com`)", secure.Rule)
	}
	if len(secure.EntryPoints) != 1 || secure.EntryPoints[0] != "websecure" {
		t.Errorf("secure router entryPoints = %v, want [websecure]", secure.EntryPoints)
	}
	if secure.TLS == nil {
		t.Errorf("secure router has no tls key; expected `tls: {}` so it attaches to websecure")
	}

	// One service, named "project-42", pointing at the upstream.
	if len(cfg.HTTP.Services) != 1 {
		t.Fatalf("expected exactly 1 service, got %d: %+v", len(cfg.HTTP.Services), cfg.HTTP.Services)
	}
	svc, ok := cfg.HTTP.Services["project-42"]
	if !ok {
		t.Fatalf("expected service %q, got services: %+v", "project-42", cfg.HTTP.Services)
	}
	if len(svc.LoadBalancer.Servers) != 1 || svc.LoadBalancer.Servers[0].URL != "http://project-42:3000" {
		t.Errorf("service loadBalancer servers = %+v, want one server with url http://project-42:3000", svc.LoadBalancer.Servers)
	}

	// Sanity check on the raw text too - both router names and the Host
	// rule should appear verbatim, since that's what a human/Traefik's
	// parser actually sees on disk.
	if !strings.Contains(string(raw), "Host(`myapp.example.com`)") {
		t.Errorf("generated file missing literal Host() rule text:\n%s", raw)
	}
}

// TestTraefikClientAddRouteOverwritesOnDomainChange exercises the
// Update-domain-change path project_service.go now relies on: since each
// project's file is keyed by project ID, not domain, "moving" a route to
// a new domain is just calling AddRoute again - the old Host() rule must
// be gone from the rewritten file.
func TestTraefikClientAddRouteOverwritesOnDomainChange(t *testing.T) {
	dir := t.TempDir()
	client := traefik.New(dir)

	if err := client.AddRoute(7, "old.example.com", "project-7:8080"); err != nil {
		t.Fatalf("AddRoute (initial): %v", err)
	}
	if err := client.AddRoute(7, "new.example.com", "project-7:8080"); err != nil {
		t.Fatalf("AddRoute (domain change): %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "project-7.yml"))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if strings.Contains(string(raw), "old.example.com") {
		t.Errorf("stale domain still present after AddRoute overwrite:\n%s", raw)
	}
	if !strings.Contains(string(raw), "new.example.com") {
		t.Errorf("new domain missing after AddRoute overwrite:\n%s", raw)
	}

	// Only one file for this project - no separate "old" file left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in dynamic dir, got %d: %v", len(entries), entries)
	}
}

// TestTraefikClientRemoveRoute covers deletion, including the no-op case
// (a project that never got a route file, or whose file was already
// removed) - RemoveRoute must not error either way.
func TestTraefikClientRemoveRoute(t *testing.T) {
	dir := t.TempDir()
	client := traefik.New(dir)

	if err := client.AddRoute(9, "gone.example.com", "project-9:80"); err != nil {
		t.Fatalf("AddRoute: %v", err)
	}
	path := filepath.Join(dir, "project-9.yml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected route file to exist before removal: %v", err)
	}

	if err := client.RemoveRoute(9); err != nil {
		t.Fatalf("RemoveRoute: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected route file to be gone after RemoveRoute, stat err: %v", err)
	}

	// Removing again (or removing a project that never had a file) is a
	// no-op, not an error.
	if err := client.RemoveRoute(9); err != nil {
		t.Fatalf("RemoveRoute on already-removed file: %v", err)
	}
	if err := client.RemoveRoute(999); err != nil {
		t.Fatalf("RemoveRoute on a project that never had a file: %v", err)
	}
}

// TestTraefikClientEnsureDir covers the boot-time directory creation the
// backend relies on (main.go calls EnsureDir once on startup).
func TestTraefikClientEnsureDir(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "nested", "dynamic")
	client := traefik.New(dir)

	if err := client.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir after EnsureDir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", dir)
	}

	// Calling it again is a no-op, not an error.
	if err := client.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir (second call): %v", err)
	}
}
