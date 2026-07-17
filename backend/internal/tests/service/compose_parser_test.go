package service_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// byName returns svcs indexed by name, so tests can assert on a specific
// service regardless of the (deterministic, alphabetical) output order.
func byName(svcs []domain.ComposeService) map[string]domain.ComposeService {
	m := make(map[string]domain.ComposeService, len(svcs))
	for _, s := range svcs {
		m[s.Name] = s
	}
	return m
}

func TestParseComposeYAMLShortSyntax(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:latest
    ports:
      - "80"
    environment:
      - FOO=bar
      - BARE
    volumes:
      - /host/path:/container/path
      - /anon-data
    depends_on:
      - db
  db:
    image: postgres:16
`
	svcs, err := service.ParseComposeYAML(yaml)
	if err != nil {
		t.Fatalf("ParseComposeYAML: %v", err)
	}
	if len(svcs) != 2 {
		t.Fatalf("got %d services, want 2", len(svcs))
	}
	byN := byName(svcs)

	web, ok := byN["web"]
	if !ok {
		t.Fatalf("missing service %q, got %v", "web", svcs)
	}
	if web.Image != "nginx:latest" {
		t.Errorf("web.Image = %q, want nginx:latest", web.Image)
	}
	wantPorts := []domain.ComposePort{{Published: "", Target: 80, Protocol: "tcp"}}
	if !reflect.DeepEqual(web.Ports, wantPorts) {
		t.Errorf("web.Ports = %+v, want %+v", web.Ports, wantPorts)
	}
	wantEnv := map[string]string{"FOO": "bar", "BARE": ""}
	if !reflect.DeepEqual(web.Environment, wantEnv) {
		t.Errorf("web.Environment = %+v, want %+v", web.Environment, wantEnv)
	}
	if len(web.Volumes) != 2 {
		t.Fatalf("web.Volumes = %+v, want 2 entries", web.Volumes)
	}
	wantVol := domain.ComposeVolume{Type: "bind", Source: "/host/path", Target: "/container/path", ReadOnly: false}
	if !reflect.DeepEqual(web.Volumes[0], wantVol) {
		t.Errorf("web.Volumes[0] = %+v, want %+v", web.Volumes[0], wantVol)
	}
	wantAnon := domain.ComposeVolume{Type: "volume", Source: "", Target: "/anon-data", ReadOnly: false}
	if !reflect.DeepEqual(web.Volumes[1], wantAnon) {
		t.Errorf("web.Volumes[1] = %+v, want %+v", web.Volumes[1], wantAnon)
	}
	if !reflect.DeepEqual(web.DependsOn, []string{"db"}) {
		t.Errorf("web.DependsOn = %v, want [db]", web.DependsOn)
	}

	if _, ok := byN["db"]; !ok {
		t.Fatalf("missing service %q, got %v", "db", svcs)
	}
}

func TestParseComposeYAMLLongSyntax(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:latest
    ports:
      - target: 80
        protocol: tcp
      - target: 9090
        protocol: udp
    environment:
      FOO: bar
      BARE: null
    volumes:
      - type: volume
        source: myvol
        target: /data
        read_only: true
    networks:
      - frontend
      - backend
    depends_on:
      db:
        condition: service_healthy
  db:
    image: postgres:16
`
	svcs, err := service.ParseComposeYAML(yaml)
	if err != nil {
		t.Fatalf("ParseComposeYAML: %v", err)
	}
	byN := byName(svcs)
	web, ok := byN["web"]
	if !ok {
		t.Fatalf("missing service %q, got %v", "web", svcs)
	}

	wantPorts := []domain.ComposePort{
		{Published: "", Target: 80, Protocol: "tcp"},
		{Published: "", Target: 9090, Protocol: "udp"},
	}
	if !reflect.DeepEqual(web.Ports, wantPorts) {
		t.Errorf("web.Ports = %+v, want %+v", web.Ports, wantPorts)
	}

	wantEnv := map[string]string{"FOO": "bar", "BARE": ""}
	if !reflect.DeepEqual(web.Environment, wantEnv) {
		t.Errorf("web.Environment = %+v, want %+v", web.Environment, wantEnv)
	}

	wantVol := []domain.ComposeVolume{{Type: "volume", Source: "myvol", Target: "/data", ReadOnly: true}}
	if !reflect.DeepEqual(web.Volumes, wantVol) {
		t.Errorf("web.Volumes = %+v, want %+v", web.Volumes, wantVol)
	}

	wantNetworks := []string{"backend", "frontend"}
	gotNetworks := append([]string{}, web.Networks...)
	sort.Strings(gotNetworks)
	if !reflect.DeepEqual(gotNetworks, wantNetworks) {
		t.Errorf("web.Networks = %v, want %v", gotNetworks, wantNetworks)
	}

	// The long depends_on map form's `condition:` is dropped - only the
	// dependency edge (the target service name) survives, per
	// Requirements ("conditions themselves are out of scope").
	if !reflect.DeepEqual(web.DependsOn, []string{"db"}) {
		t.Errorf("web.DependsOn = %v, want [db]", web.DependsOn)
	}
}

func TestParseComposeYAMLNoNetworksDeclaredGetsImplicitDefault(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:latest
`
	svcs, err := service.ParseComposeYAML(yaml)
	if err != nil {
		t.Fatalf("ParseComposeYAML: %v", err)
	}
	if len(svcs) != 1 {
		t.Fatalf("got %d services, want 1", len(svcs))
	}
	// compose semantics: a service with no explicit `networks:` still
	// attaches to the implicit "default" network.
	if !reflect.DeepEqual(svcs[0].Networks, []string{"default"}) {
		t.Errorf("Networks = %v, want [default]", svcs[0].Networks)
	}
}

func TestParseComposeYAMLRejectsBuild(t *testing.T) {
	yaml := `
services:
  web:
    build: .
`
	_, err := service.ParseComposeYAML(yaml)
	if err == nil {
		t.Fatal("expected an error for build:, got nil")
	}
	if !strings.Contains(err.Error(), "build is not supported yet; use a prebuilt image") {
		t.Errorf("error = %q, want it to mention build is not supported", err.Error())
	}
}

func TestParseComposeYAMLRejectsProfiles(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:latest
    profiles: ["dev"]
`
	_, err := service.ParseComposeYAML(yaml)
	if err == nil {
		t.Fatal("expected an error for profiles:, got nil")
	}
	if !strings.Contains(err.Error(), "profiles is not supported yet") {
		t.Errorf("error = %q, want it to mention profiles is not supported", err.Error())
	}
}

func TestParseComposeYAMLRejectsSecrets(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:latest
    secrets:
      - my_secret
secrets:
  my_secret:
    file: ./secret.txt
`
	_, err := service.ParseComposeYAML(yaml)
	if err == nil {
		t.Fatal("expected an error for secrets:, got nil")
	}
	if !strings.Contains(err.Error(), "secrets is not supported yet") {
		t.Errorf("error = %q, want it to mention secrets is not supported", err.Error())
	}
}

func TestParseComposeYAMLRejectsHealthcheck(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:latest
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost"]
      interval: 5s
`
	_, err := service.ParseComposeYAML(yaml)
	if err == nil {
		t.Fatal("expected an error for healthcheck:, got nil")
	}
	if !strings.Contains(err.Error(), "healthcheck is not supported yet") {
		t.Errorf("error = %q, want it to mention healthcheck is not supported", err.Error())
	}
}

func TestParseComposeYAMLRejectsEmptyServices(t *testing.T) {
	yaml := `
services: {}
`
	_, err := service.ParseComposeYAML(yaml)
	if err == nil {
		t.Fatal("expected an error for zero services, got nil")
	}
	if !strings.Contains(err.Error(), "no services") {
		t.Errorf("error = %q, want it to mention no services", err.Error())
	}
}

func TestParseComposeYAMLRejectsMissingDependsOnTarget(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:latest
    depends_on:
      - db
`
	_, err := service.ParseComposeYAML(yaml)
	if err == nil {
		t.Fatal("expected an error for an undefined depends_on target, got nil")
	}
	if !strings.Contains(err.Error(), "undefined service") {
		t.Errorf("error = %q, want it to mention an undefined service", err.Error())
	}
}

func TestParseComposeYAMLRejectsDuplicateServiceNames(t *testing.T) {
	// Two "web:" keys in the same services: map. compose-go's YAML
	// parser rejects a duplicate mapping key at parse time, so
	// Tamga never even sees two services with the same name - the "no
	// duplicate names" guarantee is enforced structurally by the
	// underlying parser, not by ParseComposeYAML's own code.
	yaml := `
services:
  web:
    image: nginx:1
  web:
    image: nginx:2
`
	_, err := service.ParseComposeYAML(yaml)
	if err == nil {
		t.Fatal("expected an error for a duplicate service name, got nil")
	}
	if !strings.Contains(err.Error(), "already defined") {
		t.Errorf("error = %q, want it to mention the key is already defined", err.Error())
	}
}

func TestParseComposeYAMLRejectsInvalidYAML(t *testing.T) {
	_, err := service.ParseComposeYAML("not: valid: yaml: [")
	if err == nil {
		t.Fatal("expected an error for invalid YAML, got nil")
	}
}

func TestParseComposeYAMLRejectsEveryHostPortMappingForm(t *testing.T) {
	for name, yaml := range map[string]string{
		"short host port": `services:
  web:
    image: nginx:latest
    ports: ["8080:80"]`,
		"short host address": `services:
  web:
    image: nginx:latest
    ports: ["127.0.0.1:8080:80/udp"]`,
		"short range": `services:
  web:
    image: nginx:latest
    ports: ["8080-8081:80-81"]`,
		"long published": `services:
  web:
    image: nginx:latest
    ports:
      - target: 80
        published: "8080"`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.ParseComposeYAML(yaml); err == nil || !strings.Contains(err.Error(), "must not publish host ports") {
				t.Fatalf("ParseComposeYAML() error = %v, want host-port rejection", err)
			}
		})
	}
}
