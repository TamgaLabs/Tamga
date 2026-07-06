package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	adminURL string
}

func New(adminURL string) *Client {
	return &Client{adminURL: adminURL}
}

type routeConfig struct {
	Match []matchConfig `json:"match"`
	Handle []handleConfig `json:"handle"`
}

type matchConfig struct {
	Host []string `json:"host"`
}

type handleConfig struct {
	Handler string `json:"handler"`
	Upstreams []upstreamConfig `json:"upstreams"`
}

type upstreamConfig struct {
	Dial string `json:"dial"`
}

func (c *Client) AddRoute(domain, upstream string) error {
	route := routeConfig{
		Match: []matchConfig{{Host: []string{domain}}},
		Handle: []handleConfig{{
			Handler:   "reverse_proxy",
			Upstreams: []upstreamConfig{{Dial: upstream}},
		}},
	}

	body, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("marshal route: %w", err)
	}

	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/", c.adminURL)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("caddy add route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("caddy add route: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) RemoveRoute(domain string) error {
	routes, err := c.getRoutes()
	if err != nil {
		return fmt.Errorf("get routes: %w", err)
	}

	for i, r := range routes {
		for _, m := range r.Match {
			for _, h := range m.Host {
				if h == domain {
					url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/%d", c.adminURL, i)
					req, err := http.NewRequest(http.MethodDelete, url, nil)
					if err != nil {
						return fmt.Errorf("delete route req: %w", err)
					}
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						return fmt.Errorf("caddy delete route: %w", err)
					}
					resp.Body.Close()
					return nil
				}
			}
		}
	}
	return nil
}

func (c *Client) getRoutes() ([]routeConfig, error) {
	url := fmt.Sprintf("%s/config/apps/http/servers/srv0/routes/", c.adminURL)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("get routes: %w", err)
	}
	defer resp.Body.Close()

	var routes []routeConfig
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, fmt.Errorf("decode routes: %w", err)
	}
	return routes, nil
}

// LoadConfig loads a complete Caddyfile configuration via the admin API.
// The config should be in Caddyfile format.
func (c *Client) LoadConfig(caddyfileContent []byte) error {
	url := fmt.Sprintf("%s/load?adapter=caddyfile", c.adminURL)
	resp, err := http.Post(url, "text/caddyfile", bytes.NewReader(caddyfileContent))
	if err != nil {
		return fmt.Errorf("load config request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("load config failed: status %d", resp.StatusCode)
	}
	return nil
}
