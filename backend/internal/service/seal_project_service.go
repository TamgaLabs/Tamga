package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

var projectServiceName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

var (
	ErrInvalidProjectServiceRoute  = errors.New("route must be an exact domain")
	ErrProjectServiceRouteConflict = errors.New("route domain already exists")
	ErrProjectServiceRouteNotFound = errors.New("route not found")
)

type CreateSealProjectRequest struct {
	Name      string `json:"name"`
	RemoteURL string `json:"remote_url"`
	Branch    string `json:"branch,omitempty"`
}

type CreateProjectServiceRequest struct {
	Name         string   `json:"name"`
	BuildContext string   `json:"build_context"`
	InternalPort int      `json:"internal_port"`
	Dependencies []string `json:"dependencies"`
}

type CreateProjectServiceRouteRequest struct {
	Domain string `json:"domain"`
}

type CanonicalProjectConfiguration struct {
	Project          *domain.Project   `json:"project"`
	Services         []*domain.Service `json:"services"`
	GeneratedCompose string            `json:"generated_compose"`
	BuildPermitted   bool              `json:"build_permitted"`
}

// CreateProject creates a Seal-owned repository project. The project itself is
// the source and checkout owner; no repository child entity is retained.
func (s *SealService) CreateProject(ctx context.Context, sealID int64, req CreateSealProjectRequest) (*domain.Project, error) {
	if _, err := s.db.FindSeal(sealID); err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	name := strings.TrimSpace(req.Name)
	if !safeProjectName(name) {
		return nil, fmt.Errorf("project name must be a safe name")
	}
	if strings.TrimSpace(req.RemoteURL) == "" {
		return nil, fmt.Errorf("project remote_url is required")
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = "main"
	}
	project := &domain.Project{SealID: sealID, Name: name, SourceType: domain.SourceTypeRemote, RepoURL: redactGitURL(strings.TrimSpace(req.RemoteURL)), Branch: branch, ConfigAuthority: "generated", Status: domain.ProjectStatusCreated}
	if err := s.db.CreateProject(project); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *SealService) ListProjects(ctx context.Context, sealID int64) ([]*domain.Project, error) {
	if _, err := s.db.FindSeal(sealID); err != nil {
		return nil, fmt.Errorf("find seal: %w", err)
	}
	return s.db.ListProjects(sealID)
}

func (s *SealService) ProjectCheckoutPath(sealID int64, project *domain.Project) (string, error) {
	if project.SealID != sealID || !safeProjectName(project.Name) {
		return "", fmt.Errorf("invalid project ownership")
	}
	return filepath.Join(s.cfg.DataDir, "seals", fmt.Sprintf("%d", sealID), "projects", project.Name), nil
}

// RefreshProject atomically replaces the project's owned checkout. A failed
// clone does not destroy a last known-good checkout.
func (s *SealService) RefreshProject(ctx context.Context, sealID, projectID int64) (*domain.Project, error) {
	project, err := s.db.FindProject(sealID, projectID)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	checkout, err := s.ProjectCheckoutPath(sealID, project)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(checkout), 0755); err != nil {
		return nil, fmt.Errorf("create project parent: %w", err)
	}
	temporary, err := os.MkdirTemp(filepath.Dir(checkout), "."+project.Name+".tmp-")
	if err != nil {
		return nil, fmt.Errorf("create project temporary directory: %w", err)
	}
	defer os.RemoveAll(temporary)

	project.Status = domain.ProjectStatusCloning
	if err := s.db.UpdateProject(project); err != nil {
		return nil, err
	}
	if err := projectGitClone(ctx, project.RepoURL, project.Branch, temporary); err != nil || !validProjectCheckout(temporary) {
		project.Status = domain.ProjectStatusCloneFailed
		if updateErr := s.db.UpdateProject(project); updateErr != nil {
			return nil, fmt.Errorf("record project refresh failure: %w", updateErr)
		}
		return project, nil
	}
	if err := replaceProjectCheckout(checkout, temporary); err != nil {
		project.Status = domain.ProjectStatusCloneFailed
		if updateErr := s.db.UpdateProject(project); updateErr != nil {
			return nil, fmt.Errorf("record project replacement failure: %w", updateErr)
		}
		return project, nil
	}
	project.Status = domain.ProjectStatusConfiguring
	if err := s.db.UpdateProject(project); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *SealService) DeleteProject(ctx context.Context, sealID, projectID int64) error {
	project, err := s.db.FindProject(sealID, projectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	checkout, err := s.ProjectCheckoutPath(sealID, project)
	if err != nil {
		return err
	}
	if err := s.db.DeleteProject(sealID, projectID); err != nil {
		return err
	}
	if err := os.RemoveAll(checkout); err != nil {
		return fmt.Errorf("remove project checkout: %w", err)
	}
	return nil
}

func (s *SealService) CreateProjectService(ctx context.Context, sealID, projectID int64, req CreateProjectServiceRequest) (*domain.Service, error) {
	if _, err := s.db.FindProject(sealID, projectID); err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	if !projectServiceName.MatchString(req.Name) {
		return nil, fmt.Errorf("service name must contain only letters, numbers, underscores, or hyphens and begin with a letter")
	}
	if req.InternalPort < 1 || req.InternalPort > 65535 {
		return nil, fmt.Errorf("service internal_port must be between 1 and 65535")
	}
	contextPath, err := projectBuildContext(req.BuildContext)
	if err != nil {
		return nil, err
	}
	if err := s.validateProjectServiceDependencies(sealID, projectID, req.Name, req.Dependencies); err != nil {
		return nil, err
	}
	service := &domain.Service{ProjectID: projectID, Name: req.Name, BuildContext: contextPath, InternalPort: req.InternalPort, Dependencies: normalizedProjectDependencies(req.Dependencies)}
	if err := s.db.CreateService(sealID, service); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *SealService) ListProjectServices(ctx context.Context, sealID, projectID int64) ([]*domain.Service, error) {
	if _, err := s.db.FindProject(sealID, projectID); err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	return s.db.ListServices(sealID, projectID)
}

func (s *SealService) ProjectConfiguration(ctx context.Context, sealID, projectID int64) (*CanonicalProjectConfiguration, error) {
	project, err := s.db.FindProject(sealID, projectID)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	services, err := s.db.ListServices(sealID, projectID)
	if err != nil {
		return nil, err
	}
	compose, err := generatedProjectCompose(project, services)
	if err != nil {
		return nil, err
	}
	return &CanonicalProjectConfiguration{Project: project, Services: services, GeneratedCompose: compose, BuildPermitted: len(services) > 0 && project.Status != domain.ProjectStatusCloneFailed}, nil
}

func (s *SealService) AddProjectServiceRoute(ctx context.Context, sealID, projectID, serviceID int64, req CreateProjectServiceRouteRequest) (*domain.ServiceRoute, error) {
	if _, err := s.db.FindService(sealID, projectID, serviceID); err != nil {
		return nil, fmt.Errorf("find project service: %w", err)
	}
	domainName := normalizeExactProjectDomain(req.Domain)
	if !validExactProjectDomain(domainName) {
		return nil, ErrInvalidProjectServiceRoute
	}
	route := &domain.ServiceRoute{ServiceID: serviceID, Domain: domainName}
	if err := s.db.CreateServiceRoute(sealID, projectID, route); err != nil {
		if errors.Is(err, sqlite.ErrServiceRouteDomainConflict) {
			return nil, ErrProjectServiceRouteConflict
		}
		return nil, err
	}
	if err := s.ReconcileProjectRoutes(ctx, sealID, projectID); err != nil {
		return nil, err
	}
	return route, nil
}

func (s *SealService) ListProjectServiceRoutes(ctx context.Context, sealID, projectID, serviceID int64) ([]*domain.ServiceRoute, error) {
	if _, err := s.db.FindService(sealID, projectID, serviceID); err != nil {
		return nil, fmt.Errorf("find project service: %w", err)
	}
	return s.db.ListServiceRoutes(sealID, projectID, serviceID)
}

func (s *SealService) DeleteProjectServiceRoute(ctx context.Context, sealID, projectID, serviceID, routeID int64) error {
	deleted, err := s.db.DeleteServiceRoute(sealID, projectID, serviceID, routeID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrProjectServiceRouteNotFound
	}
	return s.ReconcileProjectRoutes(ctx, sealID, projectID)
}

func (s *SealService) validateProjectServiceDependencies(sealID, projectID int64, name string, dependencies []string) error {
	services, err := s.db.ListServices(sealID, projectID)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(services))
	for _, service := range services {
		known[service.Name] = true
	}
	for _, dependency := range normalizedProjectDependencies(dependencies) {
		if dependency == name || !known[dependency] {
			return fmt.Errorf("service dependency %q is not an existing project service", dependency)
		}
	}
	return nil
}

func generatedProjectCompose(project *domain.Project, services []*domain.Service) (string, error) {
	if len(services) == 0 {
		return "services: {}\n", nil
	}
	var output strings.Builder
	output.WriteString("services:\n")
	for _, service := range services {
		contextPath := filepath.ToSlash(filepath.Join("..", "..", "projects", project.Name, service.BuildContext))
		output.WriteString(fmt.Sprintf("  %s:\n    build:\n      context: %q\n    expose:\n      - %q\n", service.Name, contextPath, fmt.Sprintf("%d", service.InternalPort)))
		if len(service.Dependencies) > 0 {
			output.WriteString("    depends_on:\n")
			for _, dependency := range service.Dependencies {
				output.WriteString(fmt.Sprintf("      - %s\n", dependency))
			}
		}
	}
	output.WriteString("networks:\n  default:\n    internal: true\n")
	return output.String(), nil
}

func safeProjectName(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

// redactGitURL keeps a usable URL while ensuring embedded credentials are
// never persisted or returned by the configuration API.
func redactGitURL(raw string) string {
	if at := strings.Index(raw, "@"); at > 0 {
		if scheme := strings.Index(raw, "://"); scheme >= 0 && at > scheme+3 {
			return raw[:scheme+3] + raw[at+1:]
		}
	}
	return raw
}

func projectBuildContext(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "."
	}
	clean := filepath.Clean(value)
	if filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("service build_context must be project-relative")
	}
	return filepath.ToSlash(clean), nil
}

func normalizedProjectDependencies(values []string) []string {
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

func normalizeExactProjectDomain(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validExactProjectDomain(value string) bool {
	if len(value) == 0 || len(value) > 253 || strings.ContainsAny(value, "/:@`*? ") {
		return false
	}
	if _, err := netip.ParseAddr(value); err == nil {
		return false
	}
	labels := strings.Split(value, ".")
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

func projectGitClone(ctx context.Context, remoteURL, branch, destination string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--single-branch", "--depth", "1", remoteURL, destination)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run()
}

func validProjectCheckout(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil && info.IsDir()
}

func replaceProjectCheckout(checkout, temporary string) error {
	backup := checkout + ".previous"
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove stale project backup: %w", err)
	}
	if err := os.Rename(checkout, backup); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("preserve current project checkout: %w", err)
	}
	if err := os.Rename(temporary, checkout); err != nil {
		if restoreErr := os.Rename(backup, checkout); restoreErr != nil && !os.IsNotExist(restoreErr) {
			return fmt.Errorf("install refreshed project: %w (restore previous checkout: %v)", err, restoreErr)
		}
		return fmt.Errorf("install refreshed project: %w", err)
	}
	if err := os.RemoveAll(backup); err != nil {
		return fmt.Errorf("remove previous project checkout: %w", err)
	}
	return nil
}
