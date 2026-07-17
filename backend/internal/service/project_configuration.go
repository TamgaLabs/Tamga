package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// ProjectConfiguration is the explicit approval boundary between cloned
// source files and a buildable Compose definition. Nothing detected here is
// persisted until SaveConfiguration is called.
type ProjectConfiguration struct {
	Sources          []*domain.ProjectSource   `json:"sources"`
	Facts            []SourceConfigurationFact `json:"facts"`
	PendingCompose   string                    `json:"pending_compose,omitempty"`
	AcceptedCompose  string                    `json:"accepted_compose,omitempty"`
	ParseErrors      []string                  `json:"parse_errors"`
	Services         []ComposeBuildService     `json:"services"`
	Recommendation   *ConfigurationSuggestion  `json:"recommendation,omitempty"`
	BuildPermitted   bool                      `json:"build_permitted"`
	EnvironmentOwner string                    `json:"environment_owner"`
}

type SourceConfigurationFact struct {
	WorkspacePath string `json:"workspace_path"`
	Dockerfile    bool   `json:"dockerfile"`
	ComposeFile   string `json:"compose_file,omitempty"`
	NextJS        bool   `json:"nextjs"`
}

type ConfigurationSuggestion struct {
	Kind string `json:"kind"`
}

type ComposeBuildService struct {
	Name       string `json:"name"`
	Context    string `json:"context"`
	Dockerfile string `json:"dockerfile"`
}

type SaveProjectConfigurationRequest struct {
	ComposeYAML         string `json:"compose_yaml"`
	AcceptDetected      bool   `json:"accept_detected"`
	ApplyNextJSTemplate bool   `json:"apply_nextjs_template"`
}

func (s *ProjectService) Configuration(ctx context.Context, projectID int64) (*ProjectConfiguration, error) {
	project, err := s.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.SourceType != domain.SourceTypeRemote {
		return nil, fmt.Errorf("project configuration requires remote sources")
	}
	result := &ProjectConfiguration{Sources: project.Sources, ParseErrors: []string{}, Services: []ComposeBuildService{}, EnvironmentOwner: "database (Compose environment is imported only on first acceptance)"}
	ready := len(project.Sources) > 0
	acceptedComposeValid := false
	var detected []string
	for _, source := range project.Sources {
		if source.Status != domain.ProjectSourceStatusReady {
			ready = false
		}
		fact, compose := s.configurationFacts(project.ID, source)
		result.Facts = append(result.Facts, fact)
		if compose != "" {
			detected = append(detected, compose)
		}
	}
	if len(detected) == 1 {
		result.PendingCompose = detected[0]
		services, err := validateBuildCompose(result.PendingCompose, project.Sources)
		if err != nil {
			result.ParseErrors = append(result.ParseErrors, err.Error())
		} else if project.ComposeYAML == "" {
			result.Services = services
		}
	}
	if len(project.Sources) == 1 && result.Facts[0].NextJS && result.PendingCompose == "" && project.ComposeYAML == "" {
		result.Recommendation = &ConfigurationSuggestion{Kind: "nextjs"}
	}
	if project.ComposeYAML != "" {
		result.AcceptedCompose = project.ComposeYAML
		services, err := validateBuildCompose(project.ComposeYAML, project.Sources)
		if err != nil {
			result.ParseErrors = append(result.ParseErrors, err.Error())
		} else {
			result.Services = services
			acceptedComposeValid = true
		}
	}
	result.BuildPermitted = ready && result.AcceptedCompose != "" && acceptedComposeValid
	return result, nil
}

func (s *ProjectService) configurationFacts(projectID int64, source *domain.ProjectSource) (SourceConfigurationFact, string) {
	base := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", projectID), filepath.FromSlash(source.WorkspacePath))
	fact := SourceConfigurationFact{WorkspacePath: source.WorkspacePath}
	if _, err := os.Stat(filepath.Join(base, "Dockerfile")); err == nil {
		fact.Dockerfile = true
	}
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"} {
		content, err := os.ReadFile(filepath.Join(base, name))
		if err == nil {
			fact.ComposeFile, _ = name, content
			break
		}
	}
	content, err := os.ReadFile(filepath.Join(base, "package.json"))
	if err == nil {
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(content, &pkg) == nil {
			_, fact.NextJS = pkg.Dependencies["next"]
			if !fact.NextJS {
				_, fact.NextJS = pkg.DevDependencies["next"]
			}
		}
	}
	return fact, string(contentOrEmpty(base, fact.ComposeFile))
}

func contentOrEmpty(base, name string) []byte {
	if name == "" {
		return nil
	}
	content, _ := os.ReadFile(filepath.Join(base, name))
	return content
}

func (s *ProjectService) SaveConfiguration(ctx context.Context, projectID int64, req SaveProjectConfigurationRequest) (*ProjectConfiguration, error) {
	project, err := s.Get(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if project.SourceType != domain.SourceTypeRemote {
		return nil, fmt.Errorf("project configuration requires remote sources")
	}
	config, err := s.Configuration(ctx, projectID)
	if err != nil {
		return nil, err
	}
	compose := strings.TrimSpace(req.ComposeYAML)
	if req.AcceptDetected {
		if compose != "" || config.PendingCompose == "" {
			return nil, fmt.Errorf("exactly one detected Compose file must be accepted")
		}
		compose = config.PendingCompose
	}
	if req.ApplyNextJSTemplate {
		if compose != "" || config.Recommendation == nil || config.Recommendation.Kind != "nextjs" {
			return nil, fmt.Errorf("Next.js template is only available for one source without detected configuration")
		}
		compose = nextJSComposeTemplate
		base := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", project.ID))
		if err := os.WriteFile(filepath.Join(base, "Dockerfile"), []byte(nextJSDockerfile), 0644); err != nil {
			return nil, fmt.Errorf("write Next.js Dockerfile: %w", err)
		}
		if err := os.WriteFile(filepath.Join(base, "compose.yaml"), []byte(compose+"\n"), 0644); err != nil {
			return nil, fmt.Errorf("write Next.js Compose file: %w", err)
		}
	}
	if compose == "" {
		return nil, fmt.Errorf("compose_yaml, detected Compose acceptance, or Next.js template is required")
	}
	if _, err := validateBuildCompose(compose, project.Sources); err != nil {
		return nil, err
	}
	firstAcceptance := project.ComposeYAML == ""
	if firstAcceptance {
		services, err := parseBuildRuntimeCompose(compose)
		if err != nil {
			return nil, err
		}
		if err := s.importComposeEnvironment(project.ID, services); err != nil {
			return nil, err
		}
	}
	project.ComposeYAML, project.ContainerID, project.Status = compose, "", domain.ProjectStatusConfiguring
	project.ConfigRevision++
	project.BuildRevision = 0
	if err := s.db.UpdateProject(project); err != nil {
		return nil, fmt.Errorf("save project configuration: %w", err)
	}
	return s.Configuration(ctx, projectID)
}

func (s *ProjectService) importComposeEnvironment(projectID int64, services []domain.ComposeService) error {
	values := make([]*domain.ServiceEnvVar, 0)
	for _, service := range services {
		for key, value := range service.Environment {
			values = append(values, &domain.ServiceEnvVar{ProjectID: projectID, ServiceName: service.Name, Key: key, Value: value})
		}
	}
	if err := s.db.ImportServiceEnvVars(values); err != nil {
		return fmt.Errorf("import Compose environment: %w", err)
	}
	return nil
}

func validateBuildCompose(yamlContent string, sources []*domain.ProjectSource) ([]ComposeBuildService, error) {
	details := types.ConfigDetails{ConfigFiles: []types.ConfigFile{{Filename: "compose.yaml", Content: []byte(yamlContent)}}, Environment: types.Mapping{}}
	project, err := loader.LoadWithContext(context.Background(), details, func(o *loader.Options) { o.SkipConsistencyCheck = true; o.SetProjectName(composeProjectName, true) })
	if err != nil {
		return nil, fmt.Errorf("parse compose YAML: %w", err)
	}
	if len(project.Services) == 0 {
		return nil, fmt.Errorf("compose file declares no services")
	}
	allowed := map[string]bool{}
	for _, source := range sources {
		allowed[source.WorkspacePath] = true
	}
	names := project.ServiceNames()
	result := make([]ComposeBuildService, 0, len(names))
	for _, name := range names {
		svc := project.Services[name]
		if err := validateNoHostPortMappings(types.Services{name: svc}); err != nil {
			return nil, err
		}
		if svc.Image != "" || svc.Build == nil {
			return nil, fmt.Errorf("service %q must use a supported build configuration, not image-only configuration", name)
		}
		contextPath, err := workspacePath(svc.Build.Context)
		if err != nil || !allowed[contextPath] {
			return nil, fmt.Errorf("service %q build context must name an owned source workspace", name)
		}
		dockerfile := svc.Build.Dockerfile
		if dockerfile == "" {
			dockerfile = "Dockerfile"
		}
		if _, err := workspacePath(dockerfile); err != nil {
			return nil, fmt.Errorf("service %q dockerfile must be workspace-relative", name)
		}
		result = append(result, ComposeBuildService{Name: name, Context: contextPath, Dockerfile: filepath.ToSlash(filepath.Clean(dockerfile))})
	}
	return result, nil
}

// parseBuildRuntimeCompose keeps the established image-only parser contract
// intact while mapping this feature's validated build services to their
// runtime settings. Image tags are supplied by Build's revision contract.
func parseBuildRuntimeCompose(yamlContent string) ([]domain.ComposeService, error) {
	details := types.ConfigDetails{ConfigFiles: []types.ConfigFile{{Filename: "compose.yaml", Content: []byte(yamlContent)}}, Environment: types.Mapping{}}
	project, err := loader.LoadWithContext(context.Background(), details, func(o *loader.Options) { o.SkipConsistencyCheck = true; o.SetProjectName(composeProjectName, true) })
	if err != nil {
		return nil, fmt.Errorf("parse compose YAML: %w", err)
	}
	names := project.ServiceNames()
	result := make([]domain.ComposeService, 0, len(names))
	for _, name := range names {
		svc := project.Services[name]
		if len(svc.Profiles) > 0 || len(svc.Secrets) > 0 || svc.HealthCheck != nil {
			return nil, fmt.Errorf("service %q uses unsupported runtime configuration", name)
		}
		if err := validateNoHostPortMappings(types.Services{name: svc}); err != nil {
			return nil, err
		}
		deps := make([]string, 0, len(svc.DependsOn))
		for dep := range svc.DependsOn {
			deps = append(deps, dep)
		}
		result = append(result, domain.ComposeService{Name: name, Ports: normalizePorts(svc.Ports), Environment: normalizeEnvironment(svc.Environment), Volumes: normalizeVolumes(svc.Volumes), Networks: normalizeNetworks(svc.Networks), DependsOn: deps})
	}
	return result, nil
}

func workspacePath(value string) (string, error) {
	clean := filepath.Clean(value)
	if value == "" || filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must be workspace-relative")
	}
	return filepath.ToSlash(clean), nil
}

const nextJSComposeTemplate = "services:\n  app:\n    build:\n      context: .\n      dockerfile: Dockerfile\n    expose:\n      - \"3000\""
const nextJSDockerfile = "FROM node:20-alpine\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci\nCOPY . .\nRUN npm run build\nEXPOSE 3000\nCMD [\"npm\", \"run\", \"start\"]\n"
