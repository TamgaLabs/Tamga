// Package traefik manages per-project Traefik dynamic-config files in the
// directory Traefik's file provider watches (providers.file.directory,
// see traefik/traefik.yml). This replaces repository/caddy's admin-API
// client: instead of POSTing route mutations to a running Caddy instance,
// the backend writes/removes one YAML file per project
// (project-<id>.yml) and Traefik's own file watcher (providers.file.watch)
// hot-reloads on change. There is no shared config to reconcile after a
// backend restart the way Caddy's LoadConfig required (see TEST-010 §2) -
// each project's file is independent.
package traefik

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Client writes/removes per-project route files under DynamicDir.
type Client struct {
	dynamicDir string
}

// New returns a Client that manages route files in dynamicDir.
func New(dynamicDir string) *Client {
	return &Client{dynamicDir: dynamicDir}
}

// dynamic-config YAML shapes. Mirrors traefik/dynamic/tamga.yml's
// split-router pattern (FEAT-023's Implementation Notes, empirically
// confirmed against a live Traefik container): a router with `tls: {}`
// does not actually attach to a non-TLS entrypoint even when that
// entrypoint is listed in its own entryPoints - Traefik only serves such a
// router on TLS-terminating entrypoints. So every route here is two
// routers (a plain one on `web`, a `-secure` one on `websecure` with
// `tls: {}`) sharing one service, rather than one dual-entrypoint router.
type dynamicConfig struct {
	HTTP httpConfig `yaml:"http"`
}

type httpConfig struct {
	Routers  map[string]routerConfig  `yaml:"routers"`
	Services map[string]serviceConfig `yaml:"services"`
}

type routerConfig struct {
	Rule        string    `yaml:"rule"`
	Service     string    `yaml:"service"`
	EntryPoints []string  `yaml:"entryPoints"`
	TLS         *struct{} `yaml:"tls,omitempty"`
}

type serviceConfig struct {
	LoadBalancer loadBalancerConfig `yaml:"loadBalancer"`
}

type loadBalancerConfig struct {
	Servers []serverConfig `yaml:"servers"`
}

type serverConfig struct {
	URL string `yaml:"url"`
}

// AddRoute writes (or overwrites) projectID's dynamic-config file, routing
// domain to upstream (a "host:port" string, e.g. "project-5:8080" - the
// same shape repository/caddy's AddRoute took). The router AND service are
// both named exactly "project-<id>" (not the domain) so Traefik's
// per-router/service Prometheus metrics
// (traefik_router_requests_total{router="project-<id>@file",...}) are
// directly attributable back to this project without a domain lookup
// (TEST-010 §4).
func (c *Client) AddRoute(projectID int64, domain, upstream string) error {
	name := fmt.Sprintf("project-%d", projectID)
	rule := fmt.Sprintf("Host(`%s`)", domain)

	cfg := dynamicConfig{
		HTTP: httpConfig{
			Routers: map[string]routerConfig{
				name: {
					Rule:        rule,
					Service:     name,
					EntryPoints: []string{"web"},
				},
				name + "-secure": {
					Rule:        rule,
					Service:     name,
					EntryPoints: []string{"websecure"},
					TLS:         &struct{}{},
				},
			},
			Services: map[string]serviceConfig{
				name: {
					LoadBalancer: loadBalancerConfig{
						Servers: []serverConfig{{URL: fmt.Sprintf("http://%s", upstream)}},
					},
				},
			},
		},
	}

	body, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal route config: %w", err)
	}

	header := fmt.Sprintf("# Managed by Tamga - do not edit by hand.\n# Regenerated on deploy/update, removed on delete, for project %d.\n", projectID)
	return c.writeFile(name, append([]byte(header), body...))
}

// RemoveRoute deletes projectID's dynamic-config file. A file that doesn't
// exist is not an error - matches callers that remove a route for a
// project which was never successfully deployed.
func (c *Client) RemoveRoute(projectID int64) error {
	name := fmt.Sprintf("project-%d", projectID)
	if err := os.Remove(c.filePath(name)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove route file: %w", err)
	}
	return nil
}

// EnsureDir creates the dynamic-config directory if it doesn't already
// exist. Safe to call repeatedly (e.g. once on backend startup, then again
// as a defensive check before every write).
func (c *Client) EnsureDir() error {
	if err := os.MkdirAll(c.dynamicDir, 0755); err != nil {
		return fmt.Errorf("ensure dynamic dir %s: %w", c.dynamicDir, err)
	}
	return nil
}

func (c *Client) filePath(name string) string {
	return filepath.Join(c.dynamicDir, name+".yml")
}

// writeFile writes content to name's file atomically: to a temp file in
// the same directory, then rename over the destination. Traefik's file
// provider watches this directory via fsnotify - a plain os.WriteFile can
// be observed mid-write (a partial/invalid YAML file momentarily on disk),
// which the watcher could try to parse and fail on. A rename is a single
// atomic filesystem operation (same directory, so same filesystem/no cross-
// device copy), so the watcher only ever sees the file complete.
func (c *Client) writeFile(name string, content []byte) error {
	if err := c.EnsureDir(); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(c.dynamicDir, "."+name+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once renamed away below

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, c.filePath(name)); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}
