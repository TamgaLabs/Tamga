package service

import (
	"reflect"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// This file is FEAT-028's deliberate exception to FEAT-021's move of
// tests into internal/tests/ (the same exception project_service_test.go
// already documents): everything in deploy_engine.go is unexported, so a
// black-box internal/tests/service test could not reach it at all.

func TestSynthesizeGitBuildService(t *testing.T) {
	envVars := []*domain.EnvVar{
		{Key: "PORT", Value: "3000"},
		{Key: "API_KEY", Value: "secret"},
	}

	svc := synthesizeGitBuildService("tamga-project-7", envVars)

	if svc.Name != gitBuildServiceName {
		t.Fatalf("expected synthesized service name %q, got %q", gitBuildServiceName, svc.Name)
	}
	if svc.Image != "tamga-project-7" {
		t.Fatalf("expected image %q, got %q", "tamga-project-7", svc.Image)
	}
	want := map[string]string{"PORT": "3000", "API_KEY": "secret"}
	if !reflect.DeepEqual(svc.Environment, want) {
		t.Fatalf("expected environment %v, got %v", want, svc.Environment)
	}
	if len(svc.Ports) != 0 {
		t.Fatalf("expected no declared ports on the synthesized service, got %v", svc.Ports)
	}

	// The synthesized service must be the sole member of its own list for
	// detectExposedService's single-service rule to expose it - assert
	// that relationship directly rather than just re-testing the two
	// functions in isolation.
	name, ok := detectExposedService([]domain.ComposeService{svc}, "")
	if !ok || name != gitBuildServiceName {
		t.Fatalf("expected the synthesized service to be exposed unconditionally, got (%q, %v)", name, ok)
	}
}

func TestSynthesizeGitBuildServiceNoEnvVars(t *testing.T) {
	svc := synthesizeGitBuildService("tamga-project-1", nil)
	if svc.Environment != nil {
		t.Fatalf("expected nil environment for a project with no env vars, got %v", svc.Environment)
	}
}

func TestApplyDatabaseEnvironmentUsesServiceValuesOverGlobals(t *testing.T) {
	services := []domain.ComposeService{{Name: "web", Environment: map[string]string{"FROM_YAML": "ignored"}}, {Name: "worker"}}
	got := applyDatabaseEnvironment(services,
		[]*domain.EnvVar{{Key: "SHARED", Value: "global"}, {Key: "GLOBAL_ONLY", Value: "yes"}},
		[]*domain.ServiceEnvVar{{ServiceName: "web", Key: "SHARED", Value: "web"}, {ServiceName: "web", Key: "WEB_ONLY", Value: "yes"}},
	)
	if want := map[string]string{"SHARED": "web", "GLOBAL_ONLY": "yes", "WEB_ONLY": "yes"}; !reflect.DeepEqual(got[0].Environment, want) {
		t.Fatalf("web environment = %v, want %v", got[0].Environment, want)
	}
	if want := map[string]string{"SHARED": "global", "GLOBAL_ONLY": "yes"}; !reflect.DeepEqual(got[1].Environment, want) {
		t.Fatalf("worker environment = %v, want %v", got[1].Environment, want)
	}
}

func TestDetectExposedServiceOverrideWins(t *testing.T) {
	services := []domain.ComposeService{
		{Name: "web", Ports: []domain.ComposePort{{Target: 8080}}},
		{Name: "redis"},
	}
	name, ok := detectExposedService(services, "redis")
	if !ok || name != "redis" {
		t.Fatalf("expected override %q to win regardless of ports, got (%q, %v)", "redis", name, ok)
	}
}

func TestDetectExposedServiceSingleServiceAlwaysExposed(t *testing.T) {
	// No declared port at all - matches the folded git-build case exactly.
	services := []domain.ComposeService{{Name: "app"}}
	name, ok := detectExposedService(services, "")
	if !ok || name != "app" {
		t.Fatalf("expected the sole service to be exposed unconditionally, got (%q, %v)", name, ok)
	}
}

func TestDetectExposedServiceExactlyOnePublishedPort(t *testing.T) {
	services := []domain.ComposeService{
		{Name: "web", Ports: []domain.ComposePort{{Target: 8080}}},
		{Name: "redis"},
		{Name: "worker"},
	}
	name, ok := detectExposedService(services, "")
	if !ok || name != "web" {
		t.Fatalf("expected 'web' (the only service with a port) to be exposed, got (%q, %v)", name, ok)
	}
}

func TestDetectExposedServiceAmbiguousNoPorts(t *testing.T) {
	services := []domain.ComposeService{{Name: "web"}, {Name: "worker"}}
	_, ok := detectExposedService(services, "")
	if ok {
		t.Fatal("expected no safe default when zero services in a multi-service stack declare a port")
	}
}

func TestDetectExposedServiceAmbiguousMultiplePorts(t *testing.T) {
	services := []domain.ComposeService{
		{Name: "web", Ports: []domain.ComposePort{{Target: 8080}}},
		{Name: "admin", Ports: []domain.ComposePort{{Target: 9090}}},
	}
	_, ok := detectExposedService(services, "")
	if ok {
		t.Fatal("expected no safe default when more than one service declares a port")
	}
}

func TestAggregateStatusAllRunning(t *testing.T) {
	got := aggregateStatus([]bool{true, true, true})
	if got != domain.ProjectStatusRunning {
		t.Fatalf("expected %q, got %q", domain.ProjectStatusRunning, got)
	}
}

func TestAggregateStatusOneDown(t *testing.T) {
	got := aggregateStatus([]bool{true, false, true})
	if got != domain.ProjectStatusError {
		t.Fatalf("expected %q, got %q", domain.ProjectStatusError, got)
	}
}

func TestAggregateStatusNoContainers(t *testing.T) {
	got := aggregateStatus(nil)
	if got != domain.ProjectStatusError {
		t.Fatalf("expected %q for a project with no service containers yet, got %q", domain.ProjectStatusError, got)
	}
}

func TestEnvMapToSliceDeterministic(t *testing.T) {
	env := map[string]string{"B": "2", "A": "1", "C": "3"}
	got := envMapToSlice(env)
	want := []string{"A=1", "B=2", "C=3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected sorted-by-key env slice %v, got %v", want, got)
	}
}

func TestComposeVolumesToMounts(t *testing.T) {
	volumes := []domain.ComposeVolume{
		{Source: "/host/data", Target: "/data"},
		{Source: "cache", Target: "/cache", ReadOnly: true},
		{Source: "", Target: "/anon"}, // anonymous volume, no source - skipped
	}
	got := composeVolumesToMounts(volumes)
	want := []string{"/host/data:/data", "cache:/cache:ro"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected mounts %v, got %v", want, got)
	}
}

func TestServiceContainerNameAndProjectNetworkName(t *testing.T) {
	if got := serviceContainerName(5, "web"); got != "project-5-web" {
		t.Fatalf("expected 'project-5-web', got %q", got)
	}
	if got := projectNetworkName(5); got != "project-net-5" {
		t.Fatalf("expected 'project-net-5', got %q", got)
	}
}

func TestToComposeServiceDeps(t *testing.T) {
	services := []domain.ComposeService{
		{Name: "web", DependsOn: []string{"db"}},
		{Name: "db"},
	}
	got := toComposeServiceDeps(services)
	want := []ComposeServiceDep{
		{Name: "web", DependsOn: []string{"db"}},
		{Name: "db"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}
