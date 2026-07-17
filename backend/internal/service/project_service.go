package service

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/traefik"
)

type ProjectService struct {
	db      *sqlite.DB
	docker  *dockerclient.Client
	traefik *traefik.Client
	cfg     config.Config
	gitCred *GitCredentialService
	// runCloneCommand is the narrow side-effect seam after cloneRepo has
	// replaced the owned checkout directory. Production uses gitCloneCommand;
	// focused lifecycle tests use it to control clone success or failure.
	runCloneCommand func(context.Context, string, string, string) error
	// runBuildImage is a narrow build seam for deterministic lifecycle tests.
	// Production delegates to the Docker-backed implementation below.
	runBuildImage func(context.Context, string, string, string) error
}

func NewProjectService(db *sqlite.DB, docker *dockerclient.Client, traefikClient *traefik.Client, cfg config.Config, gitCred *GitCredentialService) *ProjectService {
	svc := &ProjectService{db: db, docker: docker, traefik: traefikClient, cfg: cfg, gitCred: gitCred, runCloneCommand: gitCloneCommand}
	svc.runBuildImage = svc.buildImageWithDockerfile
	return svc
}

func (s *ProjectService) requireDocker() error {
	if s.docker == nil {
		return fmt.Errorf("docker daemon not available")
	}
	return nil
}

type CreateProjectRequest struct {
	Name       string            `json:"name"`
	SourceType domain.SourceType `json:"source_type"`
	RepoURL    string            `json:"repo_url"`
	Branch     string            `json:"branch,omitempty"`
	Domain     string            `json:"domain"`
	// ComposeYAML, if non-empty, makes this a compose-project create
	// (FEAT-029): the handler has already parsed/validated it (and
	// ExposedService, if set) via ParseComposeYAML before calling Create,
	// so deploy() picks it up and runs FEAT-028's compose branch instead
	// of the git clone+build path.
	ComposeYAML    string                       `json:"compose_yaml,omitempty"`
	ExposedService string                       `json:"exposed_service,omitempty"`
	Sources        []CreateProjectSourceRequest `json:"sources,omitempty"`
}

type CreateProjectSourceRequest struct {
	DisplayName   string `json:"display_name"`
	RemoteURL     string `json:"remote_url"`
	Branch        string `json:"branch,omitempty"`
	WorkspacePath string `json:"workspace_path"`
}

func (s *ProjectService) Create(ctx context.Context, req CreateProjectRequest) (*domain.Project, error) {
	if req.SourceType == "" {
		req.SourceType = domain.SourceTypeRemote
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.SourceType == domain.SourceTypeRemote && req.ComposeYAML == "" {
		sources := req.Sources
		if len(sources) == 0 {
			sources = []CreateProjectSourceRequest{{DisplayName: req.Name, RemoteURL: req.RepoURL, Branch: req.Branch, WorkspacePath: "."}}
		}
		primary := 0
		paths := make(map[string]bool, len(sources))
		for _, input := range sources {
			source, err := newProjectSource(0, input)
			if err != nil {
				return nil, err
			}
			if paths[source.WorkspacePath] {
				return nil, fmt.Errorf("source workspace_path must be unique")
			}
			paths[source.WorkspacePath] = true
			if source.WorkspacePath == "." {
				primary++
				if req.RepoURL == "" {
					req.RepoURL, req.Branch = source.RemoteURL, source.Branch
				}
			}
		}
		if primary != 1 {
			return nil, fmt.Errorf("remote project must have exactly one primary source at .")
		}
	}

	project := &domain.Project{
		Name:           req.Name,
		SourceType:     req.SourceType,
		RepoURL:        req.RepoURL,
		Branch:         req.Branch,
		Domain:         req.Domain,
		Status:         domain.ProjectStatusCreated,
		ComposeYAML:    req.ComposeYAML,
		ExposedService: req.ExposedService,
	}

	if err := s.db.CreateProject(project); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}
	if req.ComposeYAML != "" {
		services, err := ParseComposeYAML(req.ComposeYAML)
		if err != nil {
			return nil, fmt.Errorf("parse compose environment: %w", err)
		}
		if err := s.importComposeEnvironment(project.ID, services); err != nil {
			return nil, err
		}
	}

	if req.SourceType == domain.SourceTypeRemote && req.ComposeYAML == "" {
		project.Status = domain.ProjectStatusConfiguring
		if err := s.db.UpdateProject(project); err != nil {
			return nil, fmt.Errorf("set project configuring: %w", err)
		}
		sources := req.Sources
		if len(sources) == 0 {
			sources = []CreateProjectSourceRequest{{DisplayName: project.Name, RemoteURL: req.RepoURL, Branch: req.Branch, WorkspacePath: "."}}
		}
		for _, input := range sources {
			source, err := newProjectSource(project.ID, input)
			if err != nil {
				return nil, err
			}
			if err := s.db.CreateProjectSource(source); err != nil {
				return nil, err
			}
		}
		project.Sources, _ = s.db.ListProjectSources(project.ID)
	}
	slog.Info("project created", "id", project.ID, "name", project.Name)

	// Snapshot the project's initial state to return to the caller before
	// handing the original struct off to the background deploy() goroutine.
	// deploy() mutates project's fields (Status, ContainerID, ...) directly
	// and concurrently with the caller (typically an HTTP handler) reading
	// and JSON-encoding the returned value, which is an unsynchronized data
	// race (BUG-011). Returning a copy means the caller's view is always
	// exactly what was just persisted, regardless of how far deploy() has
	// progressed by the time the response is serialized.
	result := *project

	if project.Status == domain.ProjectStatusConfiguring {
		go s.cloneSources(context.Background(), project.ID)
	} else {
		go func() {
			if err := s.deploy(context.Background(), project); err != nil {
				slog.Error("deploy failed", "project_id", project.ID, "error", err)
				project.Status = domain.ProjectStatusError
				s.db.UpdateProject(project)
			}
		}()
	}

	return &result, nil
}

func newProjectSource(projectID int64, input CreateProjectSourceRequest) (*domain.ProjectSource, error) {
	if strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.RemoteURL) == "" {
		return nil, fmt.Errorf("source display_name and remote_url are required")
	}
	path, err := validateSourceWorkspacePath(input.WorkspacePath)
	if err != nil {
		return nil, err
	}
	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		branch = "main"
	}
	return &domain.ProjectSource{ProjectID: projectID, DisplayName: strings.TrimSpace(input.DisplayName), RemoteURL: redactGitURL(input.RemoteURL), Branch: branch, WorkspacePath: path, Status: domain.ProjectSourceStatusPending}, nil
}

func validateSourceWorkspacePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("source workspace_path is required")
	}
	clean := filepath.Clean(path)
	if filepath.IsAbs(path) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("source workspace_path must be relative and stay within the project workspace")
	}
	if clean == "." {
		return clean, nil
	}
	parts := strings.Split(filepath.ToSlash(clean), "/")
	if len(parts) != 2 || parts[0] != "sources" || !safeSourceName(parts[1]) {
		return "", fmt.Errorf("additional source workspace_path must be sources/<safe-name>")
	}
	return filepath.ToSlash(clean), nil
}

func safeSourceName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return value != "." && value != ".."
}

// redactGitURL keeps a usable URL while ensuring an embedded user/password is
// never persisted or returned by the configuration API.
func redactGitURL(raw string) string {
	if at := strings.Index(raw, "@"); at > 0 {
		if scheme := strings.Index(raw, "://"); scheme >= 0 && at > scheme+3 {
			return raw[:scheme+3] + raw[at+1:]
		}
	}
	return raw
}

// cloneSources owns the asynchronous source preparation lifecycle. It never
// reports git's error text because clone URLs may contain credentials.
func (s *ProjectService) cloneSources(ctx context.Context, projectID int64, only ...int64) {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		slog.Error("load project sources", "project_id", projectID, "error", err)
		return
	}
	sources, err := s.db.ListProjectSources(projectID)
	if err != nil {
		slog.Error("list project sources", "project_id", projectID, "error", err)
		return
	}
	selected := make(map[int64]bool, len(only))
	for _, id := range only {
		selected[id] = true
	}
	for _, source := range sources {
		if len(selected) > 0 && !selected[source.ID] {
			continue
		}
		source.Status = domain.ProjectSourceStatusCloning
		source.ErrorSummary = ""
		if err := s.db.UpdateProjectSource(source); err != nil {
			slog.Error("set source cloning", "project_id", projectID, "source_id", source.ID, "error", err)
			return
		}
		workDir := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", projectID), filepath.FromSlash(source.WorkspacePath))
		if err := s.cloneRepo(ctx, source.RemoteURL, source.Branch, workDir); err != nil {
			source.Status = domain.ProjectSourceStatusCloneFailed
			source.ErrorSummary = "unable to clone source"
			if updateErr := s.db.UpdateProjectSource(source); updateErr != nil {
				slog.Error("record source clone failure", "project_id", projectID, "source_id", source.ID, "error", updateErr)
			}
			project.Status = domain.ProjectStatusCloneFailed
			_ = s.db.UpdateProject(project)
			return
		}
		source.Status = domain.ProjectSourceStatusReady
		if err := s.db.UpdateProjectSource(source); err != nil {
			slog.Error("set source ready", "project_id", projectID, "source_id", source.ID, "error", err)
			return
		}
	}

	sources, err = s.db.ListProjectSources(projectID)
	if err != nil {
		return
	}
	for _, source := range sources {
		if source.Status != domain.ProjectSourceStatusReady {
			return
		}
	}
	project.Status = domain.ProjectStatusConfiguring
	_ = s.db.UpdateProject(project)
}

func (s *ProjectService) projectSources(project *domain.Project) error {
	sources, err := s.db.ListProjectSources(project.ID)
	if err != nil {
		return fmt.Errorf("list project sources: %w", err)
	}
	project.Sources = sources
	return nil
}

func (s *ProjectService) ListSources(ctx context.Context, projectID int64) ([]*domain.ProjectSource, error) {
	if _, err := s.db.FindProject(projectID); err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	return s.db.ListProjectSources(projectID)
}

func (s *ProjectService) CreateSource(ctx context.Context, projectID int64, input CreateProjectSourceRequest) (*domain.ProjectSource, error) {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	if project.SourceType != domain.SourceTypeRemote {
		return nil, fmt.Errorf("project sources are only supported for remote projects")
	}
	source, err := newProjectSource(projectID, input)
	if err != nil {
		return nil, err
	}
	if source.WorkspacePath == "." {
		return nil, fmt.Errorf("primary source already exists")
	}
	if err := s.db.CreateProjectSource(source); err != nil {
		return nil, err
	}
	project.Status, project.ContainerID = domain.ProjectStatusConfiguring, ""
	project.ConfigRevision++
	project.BuildRevision = 0
	if err := s.db.UpdateProject(project); err != nil {
		return nil, fmt.Errorf("invalidate project configuration: %w", err)
	}
	go s.cloneSources(context.Background(), projectID, source.ID)
	return source, nil
}

func (s *ProjectService) UpdateSource(ctx context.Context, projectID, sourceID int64, input CreateProjectSourceRequest) (*domain.ProjectSource, error) {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	if project.SourceType != domain.SourceTypeRemote {
		return nil, fmt.Errorf("project sources are only supported for remote projects")
	}
	source, err := s.db.FindProjectSource(projectID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("find project source: %w", err)
	}
	replacement, err := newProjectSource(projectID, input)
	if err != nil {
		return nil, err
	}
	if replacement.WorkspacePath != source.WorkspacePath {
		return nil, fmt.Errorf("source workspace_path cannot be changed")
	}
	// A remote or branch change makes the owned checkout stale.  Treat it
	// exactly like a single-source refresh: the next clone replaces that
	// directory and the project must be configured again before it can build.
	source.DisplayName, source.RemoteURL, source.Branch = replacement.DisplayName, replacement.RemoteURL, replacement.Branch
	source.Status, source.ErrorSummary = domain.ProjectSourceStatusPending, ""
	if err := s.db.UpdateProjectSource(source); err != nil {
		return nil, err
	}
	project.Status, project.ContainerID = domain.ProjectStatusConfiguring, ""
	project.ConfigRevision++
	project.BuildRevision = 0
	if err := s.db.UpdateProject(project); err != nil {
		return nil, fmt.Errorf("invalidate project configuration: %w", err)
	}
	go s.cloneSources(context.Background(), projectID, sourceID)
	return source, nil
}

func (s *ProjectService) DeleteSource(ctx context.Context, projectID, sourceID int64) error {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	source, err := s.db.FindProjectSource(projectID, sourceID)
	if err != nil {
		return fmt.Errorf("find project source: %w", err)
	}
	if source.WorkspacePath == "." {
		return fmt.Errorf("primary source cannot be deleted")
	}
	if err := s.db.DeleteProjectSource(projectID, sourceID); err != nil {
		return err
	}
	project.Status, project.ContainerID = domain.ProjectStatusConfiguring, ""
	project.ConfigRevision++
	project.BuildRevision = 0
	if err := s.db.UpdateProject(project); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", projectID), filepath.FromSlash(source.WorkspacePath)))
}

func (s *ProjectService) RefreshSource(ctx context.Context, projectID, sourceID int64) error {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	if project.SourceType != domain.SourceTypeRemote {
		return fmt.Errorf("project sources are not refreshable")
	}
	if _, err := s.db.FindProjectSource(projectID, sourceID); err != nil {
		return fmt.Errorf("find project source: %w", err)
	}
	project.Status, project.ContainerID = domain.ProjectStatusConfiguring, ""
	project.ConfigRevision++
	project.BuildRevision = 0
	if err := s.db.UpdateProject(project); err != nil {
		return fmt.Errorf("invalidate project configuration: %w", err)
	}
	go s.cloneSources(context.Background(), projectID, sourceID)
	return nil
}

func (s *ProjectService) RefreshAllSources(ctx context.Context, projectID int64) error {
	sources, err := s.ListSources(ctx, projectID)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("project has no refreshable sources")
	}
	project, err := s.db.FindProject(projectID)
	if err != nil || project.SourceType != domain.SourceTypeRemote {
		return fmt.Errorf("project sources are not refreshable")
	}
	project.Status, project.ContainerID = domain.ProjectStatusConfiguring, ""
	project.ConfigRevision++
	project.BuildRevision = 0
	if err := s.db.UpdateProject(project); err != nil {
		return fmt.Errorf("invalidate project configuration: %w", err)
	}
	go s.cloneSources(context.Background(), projectID)
	return nil
}

// deploy resolves a project's compose services - either by parsing a real
// compose_yaml (FEAT-027) or, for a legacy git-build project, by
// clone+build then folding the result into a synthesized 1-service
// compose (synthesizeGitBuildService, TEST-011 §2a) - and hands them to
// deployStack, the ONE code path both kinds of project share from there
// on (FEAT-028's core requirement).
func (s *ProjectService) deploy(ctx context.Context, project *domain.Project) error {
	if err := s.requireDocker(); err != nil {
		return err
	}

	var services []domain.ComposeService
	pullImages := false

	if project.ComposeYAML != "" {
		parsed, err := ParseComposeYAML(project.ComposeYAML)
		if err != nil {
			return fmt.Errorf("parse compose: %w", err)
		}
		services = parsed
		// A real compose project's images are pulled from a registry -
		// unlike the folded git-build case below, nothing here has
		// already built them locally.
		pullImages = true
	} else {
		workDir := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", project.ID))

		// 1. Prepare source
		project.Status = domain.ProjectStatusCloning
		s.db.UpdateProject(project)
		switch project.SourceType {
		case domain.SourceTypeLocal:
			if err := s.initRepo(ctx, workDir); err != nil {
				return fmt.Errorf("init repo: %w", err)
			}
			slog.Info("local repo initialized", "project_id", project.ID)
		default:
			if err := s.cloneRepo(ctx, project.RepoURL, project.Branch, workDir); err != nil {
				slog.Warn("clone failed, falling back to init", "project_id", project.ID, "error", err)
				if err := s.initRepo(ctx, workDir); err != nil {
					return fmt.Errorf("init repo after failed clone: %w", err)
				}
			} else {
				slog.Info("repo cloned", "project_id", project.ID)
			}
		}

		// 2. Build
		project.Status = domain.ProjectStatusBuilding
		s.db.UpdateProject(project)
		tag := fmt.Sprintf("tamga-project-%d", project.ID)
		if err := s.buildImage(ctx, tag, workDir); err != nil {
			return fmt.Errorf("build: %w", err)
		}
		slog.Info("image built", "project_id", project.ID, "tag", tag)

		envVars, err := s.db.ListEnvVars(project.ID)
		if err != nil {
			return fmt.Errorf("list env vars: %w", err)
		}
		services = []domain.ComposeService{synthesizeGitBuildService(tag, envVars)}
	}
	services, err := s.withDatabaseEnvironment(project.ID, services)
	if err != nil {
		return err
	}

	// 3. Start the (real or folded) compose stack - the unified path.
	project.Status = domain.ProjectStatusBuilding
	s.db.UpdateProject(project)
	if err := s.deployStack(ctx, project, services, pullImages); err != nil {
		return err
	}

	// 4. Done
	project.Status = domain.ProjectStatusRunning
	s.db.UpdateProject(project)

	// 5. Create deployment record
	deployment := &domain.Deployment{
		ProjectID: project.ID,
		Status:    domain.DeploymentStatusSuccess,
	}
	s.db.CreateDeployment(deployment)

	return nil
}

// Build produces every image from the accepted, owned-source Compose
// configuration. Its revision is persisted in the project row, making Deploy
// a separate no-rebuild operation with a simple stale-config guard.
func (s *ProjectService) Build(ctx context.Context, id int64) error {
	if err := s.requireDocker(); err != nil {
		return err
	}
	project, err := s.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	if project.Status == domain.ProjectStatusReadyToDeploy && project.BuildRevision == project.ConfigRevision {
		return fmt.Errorf("current build is already available")
	}
	config, err := s.Configuration(ctx, id)
	if err != nil || !config.BuildPermitted {
		return fmt.Errorf("build is not permitted until sources and configuration are valid")
	}
	// Capture before any image work. All later state writes are conditional on
	// this revision so a concurrent configuration change cannot be overwritten.
	capturedRevision := project.ConfigRevision
	if ok, err := s.db.SetBuildStateIfRevision(project.ID, capturedRevision, 0, domain.ProjectStatusBuilding); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("configuration changed before build started")
	}
	for _, build := range config.Services {
		contextDir := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", id), filepath.FromSlash(build.Context))
		if err := s.buildImageAt(ctx, buildImageTag(project.ID, capturedRevision, build.Name), contextDir, build.Dockerfile); err != nil {
			_, _ = s.db.SetBuildStateIfRevision(project.ID, capturedRevision, 0, domain.ProjectStatusBuildFailed)
			return fmt.Errorf("build service %q: %w", build.Name, err)
		}
	}
	if ok, err := s.db.SetBuildStateIfRevision(project.ID, capturedRevision, capturedRevision, domain.ProjectStatusReadyToDeploy); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("configuration changed during build")
	}
	return nil
}

func buildImageTag(projectID, revision int64, serviceName string) string {
	return fmt.Sprintf("tamga-project-%d-r%d-%s", projectID, revision, serviceName)
}

func (s *ProjectService) buildImageAt(ctx context.Context, tag, workDir, dockerfile string) error {
	return s.runBuildImage(ctx, tag, workDir, dockerfile)
}

// Deploy starts only the images created by the current successful Build.
func (s *ProjectService) Deploy(ctx context.Context, id int64) error {
	if err := s.requireDocker(); err != nil {
		return err
	}
	project, err := s.db.FindProject(id)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	if project.Status != domain.ProjectStatusReadyToDeploy || project.BuildRevision != project.ConfigRevision {
		return fmt.Errorf("deploy requires a current successful build")
	}
	services, err := parseBuildRuntimeCompose(project.ComposeYAML)
	if err != nil {
		return fmt.Errorf("parse compose: %w", err)
	}
	for i := range services {
		services[i].Image = buildImageTag(project.ID, project.BuildRevision, services[i].Name)
	}
	services, err = s.withDatabaseEnvironment(project.ID, services)
	if err != nil {
		return err
	}
	if err := s.deployStack(ctx, project, services, false); err != nil {
		return err
	}
	if err := s.writeProjectRoutes(ctx, project); err != nil {
		return err
	}
	project.Status = domain.ProjectStatusRunning
	if err := s.db.UpdateProject(project); err != nil {
		return err
	}
	return nil
}

func (s *ProjectService) SetRoutes(ctx context.Context, id int64, routes []*domain.ProjectRoute) ([]*domain.ProjectRoute, error) {
	project, err := s.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	config, err := s.Configuration(ctx, id)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, svc := range config.Services {
		known[svc.Name] = true
	}
	seen := map[string]bool{}
	for _, route := range routes {
		route.Service, route.Domain = strings.TrimSpace(route.Service), strings.ToLower(strings.TrimSpace(route.Domain))
		if !known[route.Service] || !validRouteDomain(route.Domain) {
			return nil, fmt.Errorf("route must name a configured service and domain")
		}
		if seen[route.Domain] {
			return nil, fmt.Errorf("route domains must be unique")
		}
		seen[route.Domain] = true
	}
	if err := s.db.ReplaceProjectRoutes(id, routes); err != nil {
		return nil, err
	}
	if project.Status == domain.ProjectStatusRunning {
		if err := s.writeProjectRoutes(ctx, project); err != nil {
			return nil, err
		}
	}
	return s.db.ListProjectRoutes(id)
}

func validRouteDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 || strings.ContainsAny(domain, "/:@`*? ") {
		return false
	}
	if _, err := netip.ParseAddr(domain); err == nil {
		return false
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

func (s *ProjectService) Routes(ctx context.Context, id int64) ([]*domain.ProjectRoute, error) {
	return s.db.ListProjectRoutes(id)
}

func (s *ProjectService) writeProjectRoutes(ctx context.Context, project *domain.Project) error {
	routes, err := s.db.ListProjectRoutes(project.ID)
	if err != nil {
		return err
	}
	if len(routes) == 0 {
		return s.traefik.RemoveRoute(project.ID)
	}
	containers, err := s.db.ListServiceContainers(project.ID)
	if err != nil {
		return err
	}
	byService := map[string]*domain.ServiceContainer{}
	for _, c := range containers {
		byService[c.ServiceName] = c
	}
	output := make([]traefik.Route, 0, len(routes))
	for _, route := range routes {
		c := byService[route.Service]
		if c == nil {
			return fmt.Errorf("route service %q is not deployed", route.Service)
		}
		port, err := s.docker.GetContainerPort(ctx, c.ContainerID)
		if err != nil {
			port = "80"
		}
		output = append(output, traefik.Route{Service: route.Service, Domain: route.Domain, Upstream: fmt.Sprintf("%s:%s", c.ContainerName, port)})
	}
	s.connectTraefikToNetwork(ctx, sealNetworkName(project.ID))
	return s.traefik.ReplaceRoutes(project.ID, output)
}

// deployStack is FEAT-028's unified multi-service deploy path: every
// project - whether it came from a real compose_yaml (parsed above) or a
// legacy git-build project folded into a single synthesized service
// (synthesizeGitBuildService, TEST-011 §2a) - goes through this exact same
// function, in depends_on order (FEAT-026's TopoSortServices). Shared by
// deploy() (fresh clone+build or a compose parse) and Restart() (recreate
// from already-known services, no reclone/rebuild).
//
// # Network design (closes BUG-029 - the crux of this task)
//
// Every service in the stack joins ONE per-project network,
// projectNetworkName(project.ID) ("project-net-<id>"), created here if it
// doesn't already exist. Services resolve each other by service name over
// that network's embedded DNS, exactly like real docker compose. Projects
// never share a network with each other - no project's container is ever
// attached to any network but its own - so BUG-029's cross-project
// reachability is now structurally impossible: two containers can only
// talk to each other if they share a common network.
//
// # Traefik reachability (the other half of the split with C1/BUG-028)
//
// C1 put Traefik on the flat "tamga-net" every project used to share.
// Under one-network-per-project that shared network is gone. The two
// designs TEST-011 §3 named for keeping the exposed service reachable:
//
//  1. Connect Traefik itself to each project's network (ConnectNetworks -
//     already the existing multi-network-attach primitive, precedented by
//     agent_service.go's egress-proxy attaching to every active
//     per-sandbox network).
//  2. Give the exposed service a second membership on a shared
//     "proxy-net" that only exposed services (and Traefik) join.
//
// This implementation chooses (1), connectTraefikToNetwork below: it
// reuses an existing primitive with zero new network objects (no third
// "proxy-net" to create/maintain/document), and keeps "which containers
// are on which network" answerable with a single rule - every project's
// whole stack is on exactly one network, full stop - rather than a case
// split on "is this the exposed service". Connecting Traefik to project A
// and project B's networks does NOT give A's containers a path to B's:
// Docker's bridge-network isolation is pairwise (two containers can reach
// each other only if THEY share a network), not transitive through a
// third container that happens to be on both - so cross-project isolation
// holds regardless of how many project networks Traefik itself joins
// (verified live in TEST-014). The selected-route publisher attaches
// Traefik only when at least one persisted project_route is present.
func (s *ProjectService) deployStack(ctx context.Context, project *domain.Project, services []domain.ComposeService, pullImages bool) error {
	netName := sealNetworkName(project.ID)
	if err := s.docker.EnsureNetwork(ctx, netName, false); err != nil {
		return fmt.Errorf("ensure project network: %w", err)
	}

	order, err := TopoSortServices(toComposeServiceDeps(services))
	if err != nil {
		return fmt.Errorf("order services: %w", err)
	}
	byName := make(map[string]domain.ComposeService, len(services))
	for _, svc := range services {
		byName[svc.Name] = svc
	}

	containers := make([]*domain.ServiceContainer, 0, len(services))
	for _, name := range order {
		svc := byName[name]
		containerName := serviceContainerName(project.ID, svc.Name)

		if s.docker.ContainerExists(ctx, containerName) {
			s.docker.RemoveContainer(ctx, containerName)
		}

		if pullImages {
			if err := s.docker.PullImage(ctx, svc.Image); err != nil {
				return fmt.Errorf("pull image for service %q: %w", svc.Name, err)
			}
		}

		// alias = the bare service name (e.g. "redis"), not just the full
		// container name ("project-<id>-redis") the container is also
		// always resolvable by - this is what makes inter-service
		// reachability by service name work, exactly like real docker
		// compose (TEST-014's finding: without this alias, peers get
		// NXDOMAIN resolving each other by bare service name).
		alias := []string{svc.Name}
		containerID, err := s.docker.CreateContainerOpts(ctx, containerName, svc.Image, envMapToSlice(svc.Environment), netName, composeVolumesToMounts(svc.Volumes), container.Resources{}, false, alias)
		if err != nil {
			return fmt.Errorf("create service %q: %w", svc.Name, err)
		}
		if err := s.docker.StartContainer(ctx, containerID); err != nil {
			return fmt.Errorf("start service %q: %w", svc.Name, err)
		}

		// A service's own compose-declared `networks:` (svc.Networks) are
		// deliberately NOT created as separate Docker networks here: every
		// service already joined the single project network (netName)
		// above, with its service-name alias, which already gives it full
		// intra-project reachability. A declared network on top of that is
		// redundant with the primary, not real segmentation - creating and
		// joining it too just produced a second, superfluous network with
		// every service on both (BUG-032, following BUG-029's
		// one-network-per-project design above).

		containers = append(containers, &domain.ServiceContainer{
			ProjectID:     project.ID,
			ServiceName:   svc.Name,
			ContainerID:   containerID,
			ContainerName: containerName,
			Status:        "running",
		})
		slog.Info("service container started", "project_id", project.ID, "service", svc.Name, "container_id", containerID[:12])
	}

	if err := s.db.ReplaceServiceContainers(project.ID, containers); err != nil {
		return fmt.Errorf("record service containers: %w", err)
	}

	// Keep legacy container metadata for old non-routing consumers. Public
	// routing is intentionally not derived here: persisted project_routes is
	// the sole authority and Deploy publishes it only after the stack is ready.
	exposedName, hasExposed := detectExposedService(services, project.ExposedService)
	project.ExposedService = ""
	if hasExposed {
		project.ExposedService = exposedName
	}

	// project.ContainerID keeps pointing at "the one container" legacy
	// single-container consumers (Logs, pre-FEAT-028 assumptions) still
	// read - the exposed service's container when there is one,
	// otherwise the first service started, so it's never left dangling
	// pointing at a container that no longer exists.
	project.ContainerID = ""
	for _, c := range containers {
		if hasExposed && c.ServiceName == exposedName {
			project.ContainerID = c.ContainerID
		}
	}
	if project.ContainerID == "" && len(containers) > 0 {
		project.ContainerID = containers[0].ContainerID
	}

	return nil
}

// connectTraefikToNetwork attaches the running Traefik container to a
// project's network so a route Traefik is told about can actually be
// dialed (see deployStack's design doc above). Best-effort: Traefik might
// not be running (e.g. the backend isn't running inside the full
// docker-compose stack - unit tests, or a bare `go run`), which is not a
// deploy failure, just a route that can't be reached until Traefik comes
// up and this is retried (ReconcileRoutes on the next backend restart
// re-attempts it too) - same non-fatal posture as an AddRoute failure.
func (s *ProjectService) connectTraefikToNetwork(ctx context.Context, netName string) {
	traefikName, err := s.docker.FindContainerByComposeService(ctx, "traefik")
	if err != nil {
		slog.Warn("traefik container not found, route may be unreachable", "network", netName, "error", err)
		return
	}
	// Traefik itself needs no alias here - it's located by
	// FindContainerByComposeService's label lookup, never dialed by a
	// service-name-style alias on the project network.
	if err := s.docker.NetworkConnect(ctx, netName, traefikName, nil); err != nil {
		slog.Warn("connect traefik to project network", "network", netName, "error", err)
	}
}

// disconnectTraefikFromNetwork detaches Traefik from a project's network
// before that network is removed (Delete) - a network can't be removed
// while a container is still attached to it. Best-effort: if Traefik
// can't be found there's nothing to disconnect it from either.
func (s *ProjectService) disconnectTraefikFromNetwork(ctx context.Context, netName string) {
	traefikName, err := s.docker.FindContainerByComposeService(ctx, "traefik")
	if err != nil {
		return
	}
	if err := s.docker.NetworkDisconnect(ctx, netName, traefikName); err != nil {
		slog.Warn("disconnect traefik from project network", "network", netName, "error", err)
	}
}

// Stop performs a whole-stack "down": every persisted service container is
// stopped, not removed - mirrors `docker compose down` without
// `--volumes`/`--rmi` (TEST-011 §2d). The project network and Traefik
// route are left in place so a later Restart/redeploy doesn't need to
// re-wire either.
func (s *ProjectService) Stop(ctx context.Context, id int64) error {
	if err := s.requireDocker(); err != nil {
		return err
	}
	containers, err := s.db.ListServiceContainers(id)
	if err != nil {
		return fmt.Errorf("list service containers: %w", err)
	}
	if len(containers) == 0 {
		return fmt.Errorf("no containers to stop")
	}
	// Stopping every container is teardown that must run to completion even
	// if the caller's HTTP request context is canceled mid-sweep (client
	// disconnect, timeout, etc.) - same BUG-027 class as Delete below, so it
	// gets the same fix: a detached, bounded background context instead of
	// the request ctx.
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, c := range containers {
		if err := s.docker.StopContainer(cleanupCtx, c.ContainerID); err != nil {
			slog.Warn("stop service container error", "project_id", id, "service", c.ServiceName, "container_id", c.ContainerID, "error", err)
		}
	}
	return nil
}

// resolveExposedUpstream looks up the exposed service's persisted
// project_service_containers row (project.ExposedService, resolved and
// persisted by deployStack on every deploy) and returns the
// "container-name:port" Traefik upstream string for it. ok is false when
// the project has no resolved exposed service, or that service's
// container row can't be found (e.g. the project was never successfully
// deployed). This is the one place that turns "which service is exposed"
// into "what does Traefik dial" - both ReconcileRoutes and Update's
// domain-change handling share it, so neither one re-derives (or
// silently assumes) a container name on its own. If Docker is unavailable,
// uses port 80 as a default (used by validation before route rewrite).
func (s *ProjectService) resolveExposedUpstream(ctx context.Context, project *domain.Project) (upstream string, ok bool) {
	if project.ExposedService == "" {
		return "", false
	}
	containers, err := s.db.ListServiceContainers(project.ID)
	if err != nil {
		slog.Warn("list service containers for upstream resolution", "project_id", project.ID, "error", err)
		return "", false
	}
	for _, c := range containers {
		if c.ServiceName == project.ExposedService {
			var port string
			if s.docker != nil {
				var perr error
				port, perr = s.docker.GetContainerPort(ctx, c.ContainerID)
				if perr != nil {
					port = "80"
				}
			} else {
				// Docker unavailable - use default port (validation case)
				port = "80"
			}
			return fmt.Sprintf("%s:%s", c.ContainerName, port), true
		}
	}
	return "", false
}

// exposedServiceRunning is the rebind-only running-state check that
// resolveExposedUpstream deliberately does NOT provide (BUG-033).
// resolveExposedUpstream is shared with the boot-time ReconcileRoutes and
// must keep returning ok=true for a briefly-stopped container's row so a
// boot-time reconcile doesn't drop its route - making it running-aware
// there would incorrectly skip re-adding the route for a container that
// comes back up moments later. Update's rebind validation needs a
// stricter answer than "does a row exist" though: find the target
// service's persisted project_service_containers row, then inspect
// Docker to confirm the container is actually RUNNING right now, not
// merely present from some past deploy. When Docker is unavailable (nil
// client - matches the unit tests, which never wire one up) this can't
// inspect anything, so it falls back to "does a row exist", same as
// resolveExposedUpstream, rather than failing every rebind that has no
// way to be checked.
func (s *ProjectService) exposedServiceRunning(ctx context.Context, project *domain.Project) bool {
	if project.ExposedService == "" {
		return false
	}
	containers, err := s.db.ListServiceContainers(project.ID)
	if err != nil {
		slog.Warn("list service containers for running-state check", "project_id", project.ID, "error", err)
		return false
	}
	for _, c := range containers {
		if c.ServiceName != project.ExposedService {
			continue
		}
		if s.docker == nil {
			return true
		}
		info, err := s.docker.InspectContainer(ctx, c.ContainerID)
		if err != nil {
			return false
		}
		return info.State != nil && info.State.Running
	}
	return false
}

// ReconcileRoutes re-publishes each running project's complete persisted
// selected-route set on backend startup. This removes any stale legacy
// project.Domain file and keeps project_routes as the sole public-routing
// authority.
func (s *ProjectService) ReconcileRoutes(ctx context.Context) {
	if s.docker == nil {
		return
	}
	projects, err := s.db.ListProjects()
	if err != nil {
		slog.Warn("reconcile routes: list projects failed", "error", err)
		return
	}

	for _, p := range projects {
		if p.Status != domain.ProjectStatusRunning {
			continue
		}
		if err := s.writeProjectRoutes(ctx, p); err != nil {
			slog.Warn("reconcile project routes", "project_id", p.ID, "error", err)
		}
	}
}

func (s *ProjectService) initRepo(ctx context.Context, workDir string) error {
	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("clean workdir: %w", err)
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("mkdir workdir: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "init", workDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

func (s *ProjectService) cloneRepo(ctx context.Context, repoURL, branch, workDir string) error {
	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("clean workdir: %w", err)
	}
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("mkdir workdir: %w", err)
	}

	// FEAT-008: inject the global git credential (if configured) into the
	// clone URL so private repos can be cloned without manual auth. Falls
	// back to the plain repoURL (unauthenticated) if no credential is set
	// or it can't be loaded.
	cloneURL := repoURL
	if s.gitCred != nil {
		authed, err := s.gitCred.AuthenticatedCloneURL(repoURL)
		if err != nil {
			slog.Warn("load git credential for clone, cloning unauthenticated", "error", err)
		} else {
			cloneURL = authed
		}
	}

	return s.runCloneCommand(ctx, cloneURL, branch, workDir)
}

func gitCloneCommand(ctx context.Context, cloneURL, branch, workDir string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--single-branch", "--depth", "1", cloneURL, workDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

func (s *ProjectService) buildImage(ctx context.Context, tag, workDir string) error {
	return s.buildImageWithDockerfile(ctx, tag, workDir, "Dockerfile")
}

func (s *ProjectService) buildImageWithDockerfile(ctx context.Context, tag, workDir, dockerfile string) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(workDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if fi.IsDir() {
			header.Name += "/"
			return tw.WriteHeader(header)
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return fmt.Errorf("tar workdir: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}

	return s.docker.BuildImage(ctx, tag, dockerfile, &buf)
}

func (s *ProjectService) List(ctx context.Context) ([]*domain.Project, error) {
	projects, err := s.db.ListProjects()
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if err := s.projectSources(project); err != nil {
			return nil, err
		}
	}
	return projects, nil
}

func (s *ProjectService) Get(ctx context.Context, id int64) (*domain.Project, error) {
	project, err := s.db.FindProject(id)
	if err != nil {
		return nil, err
	}
	if err := s.projectSources(project); err != nil {
		return nil, err
	}
	return project, nil
}

// Delete tears down a project's whole stack: every persisted service
// container (project_service_containers, FEAT-025), the per-project
// network, the Traefik route, then the child DB rows and the project row
// itself. Explicitly calls DeleteServiceContainersByProject rather than
// relying on the schema's ON DELETE CASCADE - this codebase does not
// enable PRAGMA foreign_keys (FEAT-025's finding), so cascade deletes
// never actually fire; DeleteEnvVarsByProject below is the existing
// precedent for the same reason. DeleteMetricsByProject (BUG-031) is the
// same story again: metric_samples/metric_latency_buckets are written by
// the C3 scraper keyed on project_id with no FK relationship to projects
// at all, so without this explicit prune a deleted project's metric rows
// become permanently orphaned (day-resolution rows in particular have no
// other retention path that would ever clean them up).
//
// # Response-before-teardown ordering (BUG-030)
//
// The route removal and DB row deletes run synchronously here, and Delete
// returns as soon as they're done - BEFORE any docker teardown happens.
// The docker sweep (stop/remove every service container, disconnect
// Traefik from the project network, remove the network) is handed off to
// teardownDockerResources on a detached goroutine instead. This ordering
// is not arbitrary: TEST-014 item 5 traced the client's DELETE getting a
// dropped connection (HTTP 000) to disconnectTraefikFromNetwork - briefly
// reconfiguring the very Traefik instance proxying the in-flight DELETE
// request drops that request's connection mid-flight, regardless of which
// context the reconfigure runs on. FEAT-028 rework 2 already moved the
// docker sweep onto a detached context so it can't be *aborted* by that
// drop (BUG-027 class - closes project-net-<id> orphaning), but the
// client-visible response was still being written only after the
// disruptive part had already run. Doing the fast, non-disruptive part
// (route removal + DB deletes) first and returning immediately means the
// handler's 204 is written, and only then does the network-disrupting
// docker work happen - by which point the response is already flushed to
// the client, so the same Traefik reconfigure blip can no longer take the
// response down with it.
func (s *ProjectService) Delete(ctx context.Context, id int64) error {
	project, err := s.db.FindProject(id)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}

	// Capture the service-container list BEFORE the DB rows are deleted
	// below - teardownDockerResources runs after they're gone (both
	// because it's handed off async and because the deletes below happen
	// first regardless), so it needs its own snapshot rather than
	// re-querying a table that will already be empty.
	var containers []*domain.ServiceContainer
	if s.docker != nil {
		containers, err = s.db.ListServiceContainers(id)
		if err != nil {
			slog.Warn("list service containers for delete", "project_id", id, "error", err)
		}
	}

	if err := s.traefik.RemoveRoute(project.ID); err != nil {
		slog.Warn("traefik remove route error", "project_id", project.ID, "domain", project.Domain, "error", err)
	}

	if err := s.db.DeleteServiceContainersByProject(id); err != nil {
		slog.Warn("delete service containers error", "project_id", id, "error", err)
	}
	if err := s.db.DeleteDeploymentsByProject(id); err != nil {
		slog.Warn("delete deployments error", "project_id", id, "error", err)
	}
	if err := s.db.DeleteEnvVarsByProject(id); err != nil {
		slog.Warn("delete env vars error", "project_id", id, "error", err)
	}
	if err := s.db.DeleteServiceEnvVarsByProject(id); err != nil {
		slog.Warn("delete service env vars error", "project_id", id, "error", err)
	}
	if err := s.db.DeleteMetricsByProject(id); err != nil {
		slog.Warn("delete metrics error", "project_id", id, "error", err)
	}

	if err := s.db.DeleteProject(id); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	workDir := filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", id))
	os.RemoveAll(workDir)

	if s.docker != nil {
		go s.teardownDockerResources(id, project.ContainerID, containers)
	}

	return nil
}

// teardownDockerResources runs the disruptive half of Delete (stop/remove
// every service container, disconnect Traefik from the project's network,
// remove that network) after Delete has already returned the client's
// response - see Delete's doc comment for why the ordering matters
// (BUG-030). Always runs on a detached, bounded background context, never
// the request context: this is must-complete cleanup exactly like
// FEAT-028 rework 2's fix for BUG-027, and by the time this goroutine
// starts the request that spawned it has already been served, so there is
// no request context left to reasonably use anyway.
func (s *ProjectService) teardownDockerResources(id int64, legacyContainerID string, containers []*domain.ServiceContainer) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, c := range containers {
		if err := s.docker.StopContainer(cleanupCtx, c.ContainerID); err != nil {
			slog.Warn("stop service container error", "project_id", id, "service", c.ServiceName, "container_id", c.ContainerID, "error", err)
		}
		if err := s.docker.RemoveContainer(cleanupCtx, c.ContainerID); err != nil {
			slog.Warn("remove service container error", "project_id", id, "service", c.ServiceName, "container_id", c.ContainerID, "error", err)
		}
	}

	// Backward compat: a project deployed before FEAT-028 has no
	// project_service_containers rows at all, only the legacy single
	// project.ContainerID - still needs its one container cleaned up.
	if len(containers) == 0 && legacyContainerID != "" {
		if err := s.docker.StopContainer(cleanupCtx, legacyContainerID); err != nil {
			slog.Warn("stop container error", "container_id", legacyContainerID, "error", err)
		}
		if err := s.docker.RemoveContainer(cleanupCtx, legacyContainerID); err != nil {
			slog.Warn("remove container error", "container_id", legacyContainerID, "error", err)
		}
	}

	// Containers must be fully removed (not merely stopped) before
	// NetworkRemove is attempted below - Docker refuses to remove a
	// network with any endpoint (running OR stopping/mid-teardown) still
	// attached, so the stop+remove sweep above has to settle first, all on
	// the same detached cleanupCtx.
	netName := sealNetworkName(id)
	s.disconnectTraefikFromNetwork(cleanupCtx, netName)
	if err := s.docker.NetworkRemove(cleanupCtx, netName); err != nil {
		slog.Warn("remove project network error", "project_id", id, "network", netName, "error", err)
	}
}

// Restart recreates the project's WHOLE stack (stop, remove, re-create,
// start every service container) rather than a plain stop+start. This is
// intentional, not an accident: Docker has no way to inject env var
// changes into an already-running container, so recreating from the
// current DB state on every restart is the only way an env var
// added/changed after a container was first created can ever actually
// take effect (BUG-021). For a legacy git-build project this re-uses the
// already-built image tag - no rebuild/reclone happens here, this is not
// a full redeploy. For a real compose project it re-parses the
// already-persisted compose_yaml - same "no rebuild" property, since
// there's nothing to build in the first place.
func (s *ProjectService) Restart(ctx context.Context, id int64) error {
	project, err := s.db.FindProject(id)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	if err := s.requireDocker(); err != nil {
		return err
	}

	var services []domain.ComposeService
	pullImages := false

	if project.ComposeYAML != "" {
		parsed, err := ParseComposeYAML(project.ComposeYAML)
		if err != nil {
			return fmt.Errorf("parse compose: %w", err)
		}
		services = parsed
		pullImages = true
	} else {
		if project.ContainerID == "" {
			return fmt.Errorf("no container to restart")
		}
		envVars, err := s.db.ListEnvVars(id)
		if err != nil {
			return fmt.Errorf("list env vars: %w", err)
		}
		tag := fmt.Sprintf("tamga-project-%d", project.ID)
		services = []domain.ComposeService{synthesizeGitBuildService(tag, envVars)}
	}
	services, err = s.withDatabaseEnvironment(project.ID, services)
	if err != nil {
		return err
	}

	if err := s.deployStack(ctx, project, services, pullImages); err != nil {
		return fmt.Errorf("redeploy stack: %w", err)
	}

	if err := s.db.UpdateProject(project); err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	slog.Info("stack recreated on restart", "project_id", project.ID)

	return nil
}

type UpdateProjectRequest struct {
	Name           *string            `json:"name,omitempty"`
	SourceType     *domain.SourceType `json:"source_type,omitempty"`
	RepoURL        *string            `json:"repo_url,omitempty"`
	Domain         *string            `json:"domain,omitempty"`
	Branch         *string            `json:"branch,omitempty"`
	ExposedService *string            `json:"exposed_service,omitempty"`
}

func (s *ProjectService) Update(ctx context.Context, id int64, req UpdateProjectRequest) (*domain.Project, error) {
	project, err := s.db.FindProject(id)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	oldExposedService := project.ExposedService

	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.SourceType != nil {
		project.SourceType = *req.SourceType
	}
	if req.RepoURL != nil {
		project.RepoURL = *req.RepoURL
	}
	if req.Domain != nil {
		project.Domain = *req.Domain
	}
	if req.Branch != nil {
		project.Branch = *req.Branch
	}
	if req.ExposedService != nil {
		project.ExposedService = *req.ExposedService
	}

	// If exposed_service is being explicitly changed to a service that has no
	// running container (when a domain exists), return an error and do NOT
	// persist the change - keeps state consistent (no half-applied rebind,
	// no broken/dangling route). Validation only applies to explicit rebind
	// requests; a Name change on a project whose exposed_service happens to
	// be down shouldn't fail (scope the error to the rebind case).
	if req.ExposedService != nil && *req.ExposedService != oldExposedService {
		// Only validate if the project has a domain (routing needs a target)
		if project.Domain != "" && project.ContainerID != "" {
			// Project is running - check the new service has an actually
			// RUNNING container to route to, not just a persisted row
			// (BUG-033: resolveExposedUpstream alone can't tell "running"
			// from "stopped but deployed" - see exposedServiceRunning's
			// doc comment for why this is a separate check).
			if !s.exposedServiceRunning(ctx, project) {
				return nil, fmt.Errorf("exposed service %q has no running container to route to", *req.ExposedService)
			}
		}
	}

	if err := s.db.UpdateProject(project); err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}

	// Legacy domain/exposed-service fields no longer control public routing;
	// only persisted project_routes are published by Deploy, SetRoutes, and
	// ReconcileRoutes. Keeping this update free of route writes prevents a
	// stale legacy value from creating an unintended public service.

	return project, nil
}

func (s *ProjectService) GetDeployments(ctx context.Context, id int64) ([]*domain.Deployment, error) {
	return s.db.ListDeployments(id)
}

func (s *ProjectService) ListEnvVars(ctx context.Context, projectID int64) ([]*domain.EnvVar, error) {
	return s.db.ListEnvVars(projectID)
}

func (s *ProjectService) withDatabaseEnvironment(projectID int64, services []domain.ComposeService) ([]domain.ComposeService, error) {
	globals, err := s.db.ListEnvVars(projectID)
	if err != nil {
		return nil, fmt.Errorf("list global env vars: %w", err)
	}
	scoped, err := s.db.ListServiceEnvVarsByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("list service env vars: %w", err)
	}
	return applyDatabaseEnvironment(services, globals, scoped), nil
}

func (s *ProjectService) supportedService(projectID int64, serviceName string) error {
	project, err := s.db.FindProject(projectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	var services []domain.ComposeService
	if project.SourceType == domain.SourceTypeRemote {
		services, err = parseBuildRuntimeCompose(project.ComposeYAML)
	} else {
		services, err = ParseComposeYAML(project.ComposeYAML)
	}
	if err != nil {
		return fmt.Errorf("parse configured services: %w", err)
	}
	for _, service := range services {
		if service.Name == serviceName {
			return nil
		}
	}
	return fmt.Errorf("service %q is not configured for project", serviceName)
}

func (s *ProjectService) ListServiceEnvVars(ctx context.Context, projectID int64, serviceName string) ([]*domain.ServiceEnvVar, error) {
	if err := s.supportedService(projectID, serviceName); err != nil {
		return nil, err
	}
	return s.db.ListServiceEnvVars(projectID, serviceName)
}

func (s *ProjectService) UpsertServiceEnvVar(ctx context.Context, projectID int64, serviceName, key, value string) (*domain.ServiceEnvVar, error) {
	if err := s.supportedService(projectID, serviceName); err != nil {
		return nil, err
	}
	ev := &domain.ServiceEnvVar{ProjectID: projectID, ServiceName: serviceName, Key: key, Value: value}
	if err := s.db.UpsertServiceEnvVar(ev); err != nil {
		return nil, err
	}
	return ev, nil
}

func (s *ProjectService) DeleteServiceEnvVar(ctx context.Context, projectID int64, serviceName string, id int64) error {
	if err := s.supportedService(projectID, serviceName); err != nil {
		return err
	}
	return s.db.DeleteServiceEnvVar(projectID, serviceName, id)
}

func (s *ProjectService) CreateEnvVar(ctx context.Context, projectID int64, key, value string) (*domain.EnvVar, error) {
	if _, err := s.db.FindProject(projectID); err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	ev := &domain.EnvVar{
		ProjectID: projectID,
		Key:       key,
		Value:     value,
	}
	if err := s.db.CreateEnvVar(ev); err != nil {
		return nil, fmt.Errorf("create env var: %w", err)
	}
	return ev, nil
}

func (s *ProjectService) DeleteEnvVar(ctx context.Context, projectID, id int64) error {
	return s.db.DeleteEnvVar(projectID, id)
}

func (s *ProjectService) Logs(ctx context.Context, id int64) (string, error) {
	project, err := s.db.FindProject(id)
	if err != nil {
		return "", fmt.Errorf("find project: %w", err)
	}
	if project.ContainerID == "" {
		return "", fmt.Errorf("no container")
	}
	if err := s.requireDocker(); err != nil {
		return "", err
	}
	return s.docker.ContainerLogs(ctx, project.ContainerID, 100)
}
