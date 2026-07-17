package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

const (
	configurationAuthorityGenerated = "generated"
	configurationAuthorityDirect    = "direct"
)

var sealServiceName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

type CreateSealServiceRequest struct {
	RepositoryID int64    `json:"repository_id"`
	Name         string   `json:"name"`
	BuildContext string   `json:"build_context"`
	InternalPort int      `json:"internal_port"`
	Dependencies []string `json:"dependencies"`
}

type SealConfigurationFact struct {
	RepositoryID  int64 `json:"repository_id"`
	Detected      bool  `json:"detected"`
	Preconfigured bool  `json:"preconfigured"`
}

type SealConfiguration struct {
	Authority      string                  `json:"authority"`
	Services       []*domain.SealService   `json:"services"`
	Facts          []SealConfigurationFact `json:"facts"`
	DirectCompose  string                  `json:"direct_compose,omitempty"`
	BuildPermitted bool                    `json:"build_permitted"`
}

type SaveSealConfigurationRequest struct {
	ComposeYAML         string `json:"compose_yaml"`
	ApplyNextJSTemplate bool   `json:"apply_nextjs_template"`
	ServiceID           int64  `json:"service_id"`
	Regenerate          bool   `json:"regenerate"`
}

func (s *SealService) ListServices(ctx context.Context, sealID int64) ([]*domain.SealService, error) {
	if _, err := s.db.FindSeal(sealID); err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	return s.db.ListSealServices(sealID)
}

func (s *SealService) CreateService(ctx context.Context, sealID int64, req CreateSealServiceRequest) (*domain.SealService, error) {
	seal, err := s.db.FindSeal(sealID)
	if err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	if !sealServiceName.MatchString(req.Name) {
		return nil, fmt.Errorf("service name must contain only letters, numbers, underscores, or hyphens and begin with a letter")
	}
	if req.InternalPort < 1 || req.InternalPort > 65535 {
		return nil, fmt.Errorf("service internal_port must be between 1 and 65535")
	}
	repository, err := s.db.FindSealRepository(sealID, req.RepositoryID)
	if err != nil {
		return nil, fmt.Errorf("find service repository: %w", err)
	}
	contextPath, err := sealBuildContext(req.BuildContext)
	if err != nil {
		return nil, err
	}
	if err := s.validateDependencies(sealID, req.Name, req.Dependencies); err != nil {
		return nil, err
	}
	service := &domain.SealService{SealID: sealID, RepositoryID: repository.ID, Name: req.Name, BuildContext: contextPath, InternalPort: req.InternalPort, Dependencies: normalizedDependencies(req.Dependencies)}
	if err := s.db.CreateSealService(service); err != nil {
		return nil, err
	}
	if seal.ConfigAuthority == configurationAuthorityGenerated {
		if err := s.materializeGeneratedConfiguration(sealID); err != nil {
			return nil, err
		}
	}
	return service, nil
}

func (s *SealService) Configuration(ctx context.Context, sealID int64) (*SealConfiguration, error) {
	seal, err := s.db.FindSeal(sealID)
	if err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	services, err := s.db.ListSealServices(sealID)
	if err != nil {
		return nil, err
	}
	repositories, err := s.db.ListSealRepositories(sealID)
	if err != nil {
		return nil, err
	}
	result := &SealConfiguration{Authority: seal.ConfigAuthority, Services: services, Facts: make([]SealConfigurationFact, 0, len(repositories))}
	for _, repository := range repositories {
		blueprint := s.nextJSBlueprint(sealID, repository)
		result.Facts = append(result.Facts, SealConfigurationFact{RepositoryID: repository.ID, Detected: blueprint.detected, Preconfigured: blueprint.preconfigured})
	}
	if seal.ConfigAuthority == configurationAuthorityDirect {
		result.DirectCompose = seal.ComposeYAML
	}
	result.BuildPermitted = len(services) > 0 && (seal.ConfigAuthority == configurationAuthorityDirect || allServicesPreconfigured(sealID, services, repositories, s))
	return result, nil
}

func allServicesPreconfigured(sealID int64, services []*domain.SealService, repositories []*domain.SealRepository, s *SealService) bool {
	byID := make(map[int64]*domain.SealRepository, len(repositories))
	for _, repository := range repositories {
		byID[repository.ID] = repository
	}
	for _, service := range services {
		if repository := byID[service.RepositoryID]; repository == nil || !s.nextJSBlueprint(sealID, repository).preconfigured {
			return false
		}
	}
	return true
}

func (s *SealService) SaveConfiguration(ctx context.Context, sealID int64, req SaveSealConfigurationRequest) (*SealConfiguration, error) {
	seal, err := s.db.FindSeal(sealID)
	if err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	if req.Regenerate {
		if req.ComposeYAML != "" || req.ApplyNextJSTemplate {
			return nil, fmt.Errorf("regenerate cannot be combined with configuration content")
		}
		seal.ConfigAuthority, seal.ComposeYAML = configurationAuthorityGenerated, ""
	} else if req.ApplyNextJSTemplate {
		if strings.TrimSpace(req.ComposeYAML) != "" || req.ServiceID <= 0 {
			return nil, fmt.Errorf("Next.js generation requires exactly one selected service")
		}
		service, err := s.db.FindSealService(sealID, req.ServiceID)
		if err != nil {
			return nil, fmt.Errorf("find Next.js service: %w", err)
		}
		repository, err := s.db.FindSealRepository(sealID, service.RepositoryID)
		if err != nil {
			return nil, fmt.Errorf("find Next.js service repository: %w", err)
		}
		if blueprint := s.nextJSBlueprint(sealID, repository); !blueprint.preconfigured {
			return nil, fmt.Errorf("selected service repository cannot use pinned Next.js blueprint: %s", blueprint.failure())
		}
		seal.ConfigAuthority, seal.ComposeYAML = configurationAuthorityGenerated, ""
	} else {
		compose := strings.TrimSpace(req.ComposeYAML)
		if compose == "" {
			return nil, fmt.Errorf("compose_yaml is required unless regenerating or generating Next.js")
		}
		if err := validateDirectSealCompose(compose); err != nil {
			return nil, err
		}
		seal.ConfigAuthority, seal.ComposeYAML = configurationAuthorityDirect, compose
	}
	if seal.ConfigAuthority == configurationAuthorityGenerated {
		if err := s.materializeGeneratedConfiguration(sealID); err != nil {
			return nil, err
		}
	}
	seal.ConfigRevision++
	seal.BuildRevision = 0
	if err := s.db.UpdateSeal(seal); err != nil {
		return nil, fmt.Errorf("save seal configuration: %w", err)
	}
	return s.Configuration(ctx, sealID)
}

func (s *SealService) validateDependencies(sealID int64, name string, dependencies []string) error {
	known, err := s.db.ListSealServices(sealID)
	if err != nil {
		return err
	}
	available := make(map[string]bool, len(known))
	for _, service := range known {
		available[service.Name] = true
	}
	for _, dependency := range normalizedDependencies(dependencies) {
		if dependency == name || !available[dependency] {
			return fmt.Errorf("service dependency %q is not an existing sibling service", dependency)
		}
	}
	return nil
}

func normalizedDependencies(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			seen[trimmed] = true
		}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sealBuildContext(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "."
	}
	clean := filepath.Clean(value)
	if filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("service build_context must be repository-relative")
	}
	return filepath.ToSlash(clean), nil
}

const (
	pinnedNextJSRepository = "MaxLeiter/maxleiter.com"
	pinnedNextJSCommit     = "add180d6f8874113d02103bc5635c04059211031"
	pinnedPNPMVersion      = "pnpm@9.15.9"
)

type nextJSBlueprintStatus struct {
	detected      bool
	preconfigured bool
	evidence      []string
}

func (b nextJSBlueprintStatus) failure() string {
	if len(b.evidence) == 0 {
		return "repository does not match the supported contract"
	}
	return strings.Join(b.evidence, "; ")
}

// nextJSBlueprint recognizes exactly the verified MaxLeiter/maxleiter.com
// checkout. Detected means the checkout declares Next.js; Preconfigured means
// its repository identity, revision, package-manager lock, and build/start
// contract are safe for Tamga's only generated blueprint.
func (s *SealService) nextJSBlueprint(sealID int64, repository *domain.SealRepository) nextJSBlueprintStatus {
	status := nextJSBlueprintStatus{}
	if repository.Status != domain.ProjectSourceStatusReady {
		status.evidence = append(status.evidence, "repository checkout is not ready")
		return status
	}
	checkout, err := s.repositoryCheckoutPath(sealID, repository)
	if err != nil {
		status.evidence = append(status.evidence, "repository checkout ownership is invalid")
		return status
	}
	content, err := os.ReadFile(filepath.Join(checkout, "package.json"))
	if err != nil {
		status.evidence = append(status.evidence, "missing package.json")
		return status
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		PackageManager  string            `json:"packageManager"`
		Scripts         map[string]string `json:"scripts"`
	}
	if json.Unmarshal(content, &pkg) != nil {
		status.evidence = append(status.evidence, "package.json is invalid JSON")
		return status
	}
	_, ok := pkg.Dependencies["next"]
	if !ok {
		_, ok = pkg.DevDependencies["next"]
	}
	status.detected = ok
	if !ok {
		status.evidence = append(status.evidence, "package.json does not declare next")
		return status
	}
	if !isPinnedNextJSRepository(repository.RemoteURL) {
		status.evidence = append(status.evidence, "repository must be MaxLeiter/maxleiter.com")
	}
	if commit, err := checkoutGitHead(checkout); err != nil || commit != pinnedNextJSCommit {
		status.evidence = append(status.evidence, "checkout must be pinned at "+pinnedNextJSCommit)
	}
	if !strings.HasPrefix(pkg.PackageManager, pinnedPNPMVersion+"+") {
		status.evidence = append(status.evidence, "package.json must pin "+pinnedPNPMVersion)
	}
	if _, err := os.Stat(filepath.Join(checkout, "pnpm-lock.yaml")); err != nil {
		status.evidence = append(status.evidence, "missing pnpm-lock.yaml")
	}
	if strings.TrimSpace(pkg.Scripts["build"]) == "" || strings.TrimSpace(pkg.Scripts["start"]) == "" {
		status.evidence = append(status.evidence, "package.json must define build and start scripts")
	}
	status.preconfigured = len(status.evidence) == 0
	return status
}

func isPinnedNextJSRepository(remoteURL string) bool {
	remote := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(remoteURL), "/"), ".git")
	remote = strings.TrimPrefix(remote, "git@github.com:")
	remote = strings.TrimPrefix(remote, "https://github.com/")
	remote = strings.TrimPrefix(remote, "http://github.com/")
	return strings.EqualFold(remote, pinnedNextJSRepository)
}

func checkoutGitHead(checkout string) (string, error) {
	head, err := os.ReadFile(filepath.Join(checkout, ".git", "HEAD"))
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(head))
	if ref, ok := strings.CutPrefix(value, "ref: "); ok {
		content, err := os.ReadFile(filepath.Join(checkout, ".git", filepath.FromSlash(ref)))
		if err != nil {
			return "", err
		}
		value = strings.TrimSpace(string(content))
	}
	return value, nil
}

func validateDirectSealCompose(content string) error {
	details := types.ConfigDetails{ConfigFiles: []types.ConfigFile{{Filename: "compose.yaml", Content: []byte(content)}}, Environment: types.Mapping{}}
	project, err := loader.LoadWithContext(context.Background(), details, func(o *loader.Options) { o.SkipConsistencyCheck = true; o.SetProjectName(composeProjectName, true) })
	if err != nil {
		return fmt.Errorf("parse compose YAML: %w", err)
	}
	if len(project.Services) == 0 {
		return fmt.Errorf("compose file declares no services")
	}
	return validateNoHostPortMappings(project.Services)
}

func (s *SealService) materializeGeneratedConfiguration(sealID int64) error {
	services, err := s.db.ListSealServices(sealID)
	if err != nil {
		return err
	}
	repositories, err := s.db.ListSealRepositories(sealID)
	if err != nil {
		return err
	}
	byID := make(map[int64]*domain.SealRepository, len(repositories))
	for _, repository := range repositories {
		byID[repository.ID] = repository
	}
	compose, needsNextJSDockerfile, err := s.generatedCompose(sealID, services, byID)
	if err != nil {
		return err
	}
	return s.writeGeneratedCompose(sealID, compose, needsNextJSDockerfile)
}

// generatedCompose projects validated declarations without publishing host
// ports. Compose and generated Dockerfiles stay in the Seal-owned .tamga
// directory; build contexts only reference owned repository checkouts.
func (s *SealService) generatedCompose(sealID int64, services []*domain.SealService, repositories map[int64]*domain.SealRepository) (string, bool, error) {
	if len(services) == 0 {
		return "services: {}\n", false, nil
	}
	var output strings.Builder
	output.WriteString("services:\n")
	needsNextJSDockerfile := false
	for _, service := range services {
		repository := repositories[service.RepositoryID]
		if repository == nil {
			return "", false, fmt.Errorf("find generated service repository %d", service.RepositoryID)
		}
		contextPath := filepath.ToSlash(filepath.Join("..", "..", repository.WorkspacePath, service.BuildContext))
		output.WriteString(fmt.Sprintf("  %s:\n    build:\n      context: %q\n", service.Name, contextPath))
		if s.nextJSBlueprint(sealID, repository).preconfigured {
			dockerfilePath, err := filepath.Rel(filepath.FromSlash(filepath.Join(repository.WorkspacePath, service.BuildContext)), filepath.FromSlash(filepath.Join(".tamga", "generated", "Dockerfile")))
			if err != nil {
				return "", false, fmt.Errorf("resolve generated Dockerfile path: %w", err)
			}
			output.WriteString(fmt.Sprintf("      dockerfile: %q\n", filepath.ToSlash(dockerfilePath)))
			needsNextJSDockerfile = true
		}
		output.WriteString(fmt.Sprintf("    expose:\n      - %q\n", fmt.Sprintf("%d", service.InternalPort)))
		if len(service.Dependencies) > 0 {
			output.WriteString("    depends_on:\n")
			for _, dependency := range service.Dependencies {
				output.WriteString(fmt.Sprintf("      - %s\n", dependency))
			}
		}
	}
	output.WriteString("networks:\n  default:\n    internal: true\n")
	return output.String(), needsNextJSDockerfile, nil
}

func (s *SealService) writeGeneratedCompose(sealID int64, compose string, needsNextJSDockerfile bool) error {
	generated := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", sealID), ".tamga", "generated")
	if err := os.MkdirAll(generated, 0755); err != nil {
		return fmt.Errorf("create generated configuration directory: %w", err)
	}
	dockerfile := filepath.Join(generated, "Dockerfile")
	if needsNextJSDockerfile {
		if err := os.WriteFile(dockerfile, []byte(generatedNextJSDockerfile), 0644); err != nil {
			return fmt.Errorf("write generated Next.js Dockerfile: %w", err)
		}
	} else if err := os.Remove(dockerfile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale generated Next.js Dockerfile: %w", err)
	}
	if err := os.WriteFile(filepath.Join(generated, "compose.yaml"), []byte(compose), 0644); err != nil {
		return fmt.Errorf("write generated Next.js Compose: %w", err)
	}
	return nil
}

const generatedNextJSDockerfile = "FROM node:20-alpine\nWORKDIR /app\nCOPY package.json pnpm-lock.yaml ./\nRUN corepack enable && corepack prepare pnpm@9.15.9 --activate && pnpm install --frozen-lockfile\nCOPY . .\nRUN pnpm build\nENV HOSTNAME=0.0.0.0\nEXPOSE 3000\nCMD [\"pnpm\", \"start\"]\n"
