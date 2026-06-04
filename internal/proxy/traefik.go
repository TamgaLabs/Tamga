package proxy

import (
	"fmt"
)

type TraefikConfig struct {
	Domain       string
	InternalPort string
	ProjectName  string
}

type Labels map[string]string

func GenerateLabels(cfg TraefikConfig) Labels {
	routerName := sanitize(cfg.ProjectName)
	serviceName := routerName

	return Labels{
		"traefik.enable": "true",

		"traefik.http.routers." + routerName + ".rule":                                  "Host(`" + cfg.Domain + "`)",
		"traefik.http.routers." + routerName + ".entrypoints":                           "websecure",
		"traefik.http.routers." + routerName + ".tls":                                   "true",
		"traefik.http.routers." + routerName + ".tls.certresolver":                      "letsencrypt",
		"traefik.http.services." + serviceName + ".loadbalancer.server.port":            cfg.InternalPort,
		"traefik.http.routers." + routerName + ".middlewares":                           "redirect-https@file",
	}
}

func GenerateHTTPLabels(cfg TraefikConfig) Labels {
	routerName := sanitize(cfg.ProjectName)
	serviceName := routerName

	return Labels{
		"traefik.enable": "true",

		"traefik.http.routers." + routerName + ".rule":                      "Host(`" + cfg.Domain + "`)",
		"traefik.http.routers." + routerName + ".entrypoints":               "web",
		"traefik.http.services." + serviceName + ".loadbalancer.server.port": cfg.InternalPort,
	}
}

func GenerateLabelsWithMiddleware(cfg TraefikConfig, middlewares []string) Labels {
	labels := GenerateLabels(cfg)
	routerName := sanitize(cfg.ProjectName)

	if len(middlewares) > 0 {
		mwStr := ""
		for i, m := range middlewares {
			if i > 0 {
				mwStr += ","
			}
			mwStr += m
		}
		labels["traefik.http.routers."+routerName+".middlewares"] = mwStr
	}

	return labels
}

func MergeLabels(base, extra Labels) Labels {
	result := make(Labels, len(base)+len(extra))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}

func (l Labels) ToSlice() []string {
	s := make([]string, 0, len(l))
	for k, v := range l {
		s = append(s, fmt.Sprintf("%s=%s", k, v))
	}
	return s
}

func sanitize(name string) string {
	// Remove characters that are invalid in Traefik router/service names
	b := make([]byte, 0, len(name))
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			b = append(b, c)
		} else if c == '_' || c == '.' {
			b = append(b, '-')
		}
	}
	if len(b) == 0 {
		return "app"
	}
	return string(b)
}
