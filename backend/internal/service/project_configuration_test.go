package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
)

func TestProjectConfigurationApprovalAndSafePaths(t *testing.T) {
	svc, cfg := newTestProjectService(t)
	project := &domain.Project{Name: "web", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatal(err)
	}
	source := &domain.ProjectSource{ProjectID: project.ID, DisplayName: "web", RemoteURL: "https://example.test/web.git", Branch: "main", WorkspacePath: ".", Status: domain.ProjectSourceStatusReady}
	if err := svc.db.CreateProjectSource(source); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(cfg.DataDir, "projects", fmt.Sprintf("%d", project.ID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "package.json"), []byte(`{"dependencies":{"next":"15"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := svc.Configuration(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if config.AcceptedCompose != "" || config.Recommendation == nil || config.Recommendation.Kind != "nextjs" || config.BuildPermitted {
		t.Fatalf("expected non-applied Next.js recommendation, got %+v", config)
	}
	config, err = svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ApplyNextJSTemplate: true})
	if err != nil {
		t.Fatal(err)
	}
	if !config.BuildPermitted || len(config.Services) != 1 || config.Services[0].Context != "." {
		t.Fatalf("expected accepted build configuration, got %+v", config)
	}
	for _, name := range []string{"Dockerfile", "compose.yaml"} {
		if _, err := os.Stat(filepath.Join(workspace, name)); err != nil {
			t.Fatalf("expected generated %s: %v", name, err)
		}
	}
	for _, compose := range []string{
		"services:\n  app:\n    image: nginx",
		"services:\n  app:\n    build:\n      context: ../escape",
		"services:\n  app:\n    build:\n      context: sources/missing",
	} {
		if _, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: compose}); err == nil {
			t.Fatalf("expected unsafe or unsupported compose rejection: %q", compose)
		}
	}
}

func TestProjectConfigurationDetectedComposeAndMultiSourceNoTemplate(t *testing.T) {
	svc, cfg := newTestProjectService(t)
	project := &domain.Project{Name: "multi", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatal(err)
	}
	for _, source := range []*domain.ProjectSource{
		{ProjectID: project.ID, DisplayName: "web", RemoteURL: "https://example.test/web.git", Branch: "main", WorkspacePath: ".", Status: domain.ProjectSourceStatusReady},
		{ProjectID: project.ID, DisplayName: "worker", RemoteURL: "https://example.test/worker.git", Branch: "main", WorkspacePath: "sources/worker", Status: domain.ProjectSourceStatusReady},
	} {
		if err := svc.db.CreateProjectSource(source); err != nil {
			t.Fatal(err)
		}
	}
	workspace := filepath.Join(cfg.DataDir, "projects", fmt.Sprintf("%d", project.ID))
	if err := os.MkdirAll(filepath.Join(workspace, "sources", "worker"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "package.json"), []byte(`{"dependencies":{"next":"15"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	detected := "services:\n  app:\n    build:\n      context: sources/worker\n      dockerfile: Dockerfile"
	if err := os.WriteFile(filepath.Join(workspace, "compose.yaml"), []byte(detected), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := svc.Configuration(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if config.PendingCompose != detected || config.Recommendation != nil || config.BuildPermitted {
		t.Fatalf("expected detected-but-pending multi-source config, got %+v", config)
	}
	config, err = svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{AcceptDetected: true})
	if err != nil || !config.BuildPermitted || len(config.Services) != 1 || config.Services[0].Context != "sources/worker" {
		t.Fatalf("accept detected compose: config=%+v err=%v", config, err)
	}
}

func TestProjectConfigurationRejectsEscapingDockerfilePath(t *testing.T) {
	svc, _ := newTestProjectService(t)
	project := &domain.Project{Name: "web", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatal(err)
	}
	if err := svc.db.CreateProjectSource(&domain.ProjectSource{ProjectID: project.ID, DisplayName: "web", RemoteURL: "https://example.test/web.git", Branch: "main", WorkspacePath: ".", Status: domain.ProjectSourceStatusReady}); err != nil {
		t.Fatal(err)
	}

	_, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: "services:\n  app:\n    build:\n      context: .\n      dockerfile: ../Dockerfile"})
	if err == nil || !strings.Contains(err.Error(), "dockerfile must be workspace-relative") {
		t.Fatalf("expected source-relative Dockerfile rejection, got %v", err)
	}
}

func TestBuildPermittedIgnoresMalformedPendingComposeWhenAcceptedComposeIsValid(t *testing.T) {
	svc, cfg := newTestProjectService(t)
	project := &domain.Project{Name: "web", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatal(err)
	}
	if err := svc.db.CreateProjectSource(&domain.ProjectSource{ProjectID: project.ID, DisplayName: "web", RemoteURL: "https://example.test/web.git", Branch: "main", WorkspacePath: ".", Status: domain.ProjectSourceStatusReady}); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(cfg.DataDir, "projects", fmt.Sprintf("%d", project.ID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "compose.yaml"), []byte("services: ["), 0644); err != nil {
		t.Fatal(err)
	}
	accepted := "services:\n  web:\n    build:\n      context: .\n"
	if _, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: accepted}); err != nil {
		t.Fatalf("save accepted configuration: %v", err)
	}

	config, err := svc.Configuration(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.ParseErrors) == 0 || !config.BuildPermitted {
		t.Fatalf("malformed pending Compose blocked valid accepted configuration: %+v", config)
	}

	svc.docker = &dockerclient.Client{}
	svc.runBuildImage = func(context.Context, string, string, string) error { return nil }
	if err := svc.Build(context.Background(), project.ID); err != nil {
		t.Fatalf("Build with valid accepted configuration: %v", err)
	}
}

func TestBuildDoesNotRestoreConfigurationChangedDuringBuild(t *testing.T) {
	svc, cfg := newTestProjectService(t)
	project := &domain.Project{Name: "web", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatal(err)
	}
	if err := svc.db.CreateProjectSource(&domain.ProjectSource{ProjectID: project.ID, DisplayName: "web", RemoteURL: "https://example.test/web.git", Branch: "main", WorkspacePath: ".", Status: domain.ProjectSourceStatusReady}); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(cfg.DataDir, "projects", fmt.Sprintf("%d", project.ID))
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "Dockerfile"), []byte("FROM scratch\n"), 0644); err != nil {
		t.Fatal(err)
	}
	compose := "services:\n  web:\n    build:\n      context: .\n"
	if _, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: compose}); err != nil {
		t.Fatalf("save configuration: %v", err)
	}

	// Build only needs a non-nil client to pass its availability gate; the
	// injected seam below keeps this test independent of a Docker daemon.
	svc.docker = &dockerclient.Client{}
	started := make(chan struct{})
	release := make(chan struct{})
	svc.runBuildImage = func(context.Context, string, string, string) error {
		close(started)
		<-release
		return nil
	}
	buildResult := make(chan error, 1)
	go func() { buildResult <- svc.Build(context.Background(), project.ID) }()
	<-started
	if _, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: compose}); err != nil {
		t.Fatalf("concurrent configuration update: %v", err)
	}
	close(release)
	if err := <-buildResult; err == nil || !strings.Contains(err.Error(), "configuration changed during build") {
		t.Fatalf("Build error = %v, want stale configuration error", err)
	}

	got, err := svc.db.FindProject(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ConfigRevision != 2 || got.BuildRevision != 0 || got.Status != domain.ProjectStatusConfiguring {
		t.Fatalf("stale build overwrote current config: %+v", got)
	}
}

func TestBuildBuildsEveryConfiguredServiceAndDeployRequiresThatBuild(t *testing.T) {
	svc, _ := newTestProjectService(t)
	project := &domain.Project{Name: "stack", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatal(err)
	}
	if err := svc.db.CreateProjectSource(&domain.ProjectSource{ProjectID: project.ID, DisplayName: "stack", RemoteURL: "https://example.test/stack.git", Branch: "main", WorkspacePath: ".", Status: domain.ProjectSourceStatusReady}); err != nil {
		t.Fatal(err)
	}
	compose := "services:\n  web:\n    build:\n      context: .\n  worker:\n    build:\n      context: ."
	if _, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: compose}); err != nil {
		t.Fatalf("save configuration: %v", err)
	}
	svc.docker = &dockerclient.Client{}
	if err := svc.Deploy(context.Background(), project.ID); err == nil || !strings.Contains(err.Error(), "current successful build") {
		t.Fatalf("Deploy before Build error = %v, want current-build gate", err)
	}
	var tags []string
	svc.runBuildImage = func(_ context.Context, tag, _, _ string) error {
		tags = append(tags, tag)
		return nil
	}
	if err := svc.Build(context.Background(), project.ID); err != nil {
		t.Fatalf("build services: %v", err)
	}
	if len(tags) != 2 || tags[0] != buildImageTag(project.ID, 1, "web") || tags[1] != buildImageTag(project.ID, 1, "worker") {
		t.Fatalf("build tags = %v", tags)
	}
	stored, err := svc.db.FindProject(project.ID)
	if err != nil || stored.Status != domain.ProjectStatusReadyToDeploy || stored.BuildRevision != stored.ConfigRevision {
		t.Fatalf("successful build state = %+v, err=%v", stored, err)
	}
}

func TestServiceEnvironmentImportsOnceAndValidatesConfiguredService(t *testing.T) {
	svc, _ := newTestProjectService(t)
	project := &domain.Project{Name: "web", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatal(err)
	}
	if err := svc.db.CreateProjectSource(&domain.ProjectSource{ProjectID: project.ID, DisplayName: "web", RemoteURL: "https://example.test/web.git", Branch: "main", WorkspacePath: ".", Status: domain.ProjectSourceStatusReady}); err != nil {
		t.Fatal(err)
	}
	first := "services:\n  web:\n    build:\n      context: .\n    environment:\n      SHARED: yaml\n      FIRST: one"
	if _, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: first}); err != nil {
		t.Fatalf("save first configuration: %v", err)
	}
	values, err := svc.ListServiceEnvVars(context.Background(), project.ID, "web")
	if err != nil || len(values) != 2 {
		t.Fatalf("imported service values = %+v, err=%v", values, err)
	}
	if _, err := svc.UpsertServiceEnvVar(context.Background(), project.ID, "web", "SHARED", "database"); err != nil {
		t.Fatalf("upsert service value: %v", err)
	}
	second := "services:\n  web:\n    build:\n      context: .\n    environment:\n      SHARED: changed\n      SECOND: two"
	if _, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: second}); err != nil {
		t.Fatalf("save later configuration: %v", err)
	}
	values, err = svc.ListServiceEnvVars(context.Background(), project.ID, "web")
	if err != nil || len(values) != 2 {
		t.Fatalf("later YAML was imported: %+v, err=%v", values, err)
	}
	for _, value := range values {
		if value.Key == "SHARED" && value.Value != "database" {
			t.Fatalf("database value was overwritten: %+v", values)
		}
	}
	if _, err := svc.UpsertServiceEnvVar(context.Background(), project.ID, "missing", "KEY", "value"); err == nil {
		t.Fatal("expected unknown service rejection")
	}
}

func TestSetRoutesAcceptsConfiguredServiceAndRejectsPathRouting(t *testing.T) {
	svc, _ := newTestProjectService(t)
	project := &domain.Project{Name: "web", SourceType: domain.SourceTypeRemote, Status: domain.ProjectStatusConfiguring}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatal(err)
	}
	if err := svc.db.CreateProjectSource(&domain.ProjectSource{ProjectID: project.ID, DisplayName: "web", RemoteURL: "https://example.test/web.git", Branch: "main", WorkspacePath: ".", Status: domain.ProjectSourceStatusReady}); err != nil {
		t.Fatal(err)
	}
	compose := "services:\n  web:\n    build:\n      context: .\n    ports:\n      - \"8080:8080\"\n  worker:\n    build:\n      context: ."
	if _, err := svc.SaveConfiguration(context.Background(), project.ID, SaveProjectConfigurationRequest{ComposeYAML: compose}); err != nil {
		t.Fatalf("save configuration: %v", err)
	}
	routes, err := svc.SetRoutes(context.Background(), project.ID, []*domain.ProjectRoute{{Service: "web", Domain: "App.Example.Test"}})
	if err != nil || len(routes) != 1 || routes[0].Domain != "app.example.test" {
		t.Fatalf("set configured route = %+v, err=%v", routes, err)
	}
	if _, err := svc.SetRoutes(context.Background(), project.ID, []*domain.ProjectRoute{{Service: "web", Domain: "app.example.test/admin"}}); err == nil {
		t.Fatal("expected path-routing domain rejection")
	}
	if _, err := svc.SetRoutes(context.Background(), project.ID, []*domain.ProjectRoute{{Service: "missing", Domain: "missing.example.test"}}); err == nil {
		t.Fatal("expected unconfigured service rejection")
	}
}
