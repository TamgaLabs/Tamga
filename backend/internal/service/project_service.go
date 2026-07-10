package service

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

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
}

func NewProjectService(db *sqlite.DB, docker *dockerclient.Client, traefikClient *traefik.Client, cfg config.Config, gitCred *GitCredentialService) *ProjectService {
	return &ProjectService{db: db, docker: docker, traefik: traefikClient, cfg: cfg, gitCred: gitCred}
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
}

func (s *ProjectService) Create(ctx context.Context, req CreateProjectRequest) (*domain.Project, error) {
	if req.SourceType == "" {
		req.SourceType = domain.SourceTypeRemote
	}
	if req.Branch == "" {
		req.Branch = "main"
	}

	project := &domain.Project{
		Name:       req.Name,
		SourceType: req.SourceType,
		RepoURL:    req.RepoURL,
		Branch:     req.Branch,
		Domain:     req.Domain,
		Status:     domain.ProjectStatusCreated,
	}

	if err := s.db.CreateProject(project); err != nil {
		return nil, fmt.Errorf("create project: %w", err)
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

	go func() {
		if err := s.deploy(context.Background(), project); err != nil {
			slog.Error("deploy failed", "project_id", project.ID, "error", err)
			project.Status = domain.ProjectStatusError
			s.db.UpdateProject(project)
		}
	}()

	return &result, nil
}

func (s *ProjectService) deploy(ctx context.Context, project *domain.Project) error {
	if err := s.requireDocker(); err != nil {
		return err
	}
	workDir := filepath.Join(s.cfg.DataDir, "projects", fmt.Sprintf("%d", project.ID))

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

	// 3. Run container
	containerName := fmt.Sprintf("project-%d", project.ID)
	if s.docker.ContainerExists(ctx, containerName) {
		s.docker.RemoveContainer(ctx, containerName)
	}

	// Ensure the shared project network exists before attaching to it.
	// Nothing else in the codebase creates "tamga-net" (agent_service.go's
	// EnsureNetwork calls are for the isolated, internal, per-project agent
	// sandbox networks - a different thing entirely, see BUG-020). Unlike
	// those sandbox networks, this one is not internal: project containers
	// generally need real outbound access (installing packages, calling
	// external APIs, etc.), the same way the compose-managed tamga-network
	// isn't internal either.
	if err := s.docker.EnsureNetwork(ctx, "tamga-net", false); err != nil {
		return fmt.Errorf("ensure project network: %w", err)
	}

	envVars, err := s.db.ListEnvVars(project.ID)
	if err != nil {
		return fmt.Errorf("list env vars: %w", err)
	}

	containerID, err := s.docker.CreateContainer(ctx, containerName, tag, envVarsToSlice(envVars), "tamga-net")
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	if err := s.docker.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	project.ContainerID = containerID
	slog.Info("container started", "project_id", project.ID, "container_id", containerID[:12])

	// 4. Register Traefik route
	port, err := s.docker.GetContainerPort(ctx, containerID)
	if err != nil {
		port = "80"
	}
	upstream := fmt.Sprintf("%s:%s", containerName, port)
	if err := s.traefik.AddRoute(project.ID, project.Domain, upstream); err != nil {
		slog.Warn("traefik route failed", "project_id", project.ID, "domain", project.Domain, "error", err)
		// non-fatal: container is running, route can be added manually
	}
	slog.Info("traefik route added", "project_id", project.ID, "domain", project.Domain, "upstream", upstream)

	// 5. Done
	project.Status = domain.ProjectStatusRunning
	s.db.UpdateProject(project)

	// 6. Create deployment record
	deployment := &domain.Deployment{
		ProjectID: project.ID,
		Status:    domain.DeploymentStatusSuccess,
	}
	s.db.CreateDeployment(deployment)

	return nil
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

	cmd := exec.CommandContext(ctx, "git", "clone", "--branch", branch, "--single-branch", "--depth", "1", cloneURL, workDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

func (s *ProjectService) buildImage(ctx context.Context, tag, workDir string) error {
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

	return s.docker.BuildImage(ctx, tag, "Dockerfile", &buf)
}

func (s *ProjectService) List(ctx context.Context) ([]*domain.Project, error) {
	return s.db.ListProjects()
}

func (s *ProjectService) Get(ctx context.Context, id int64) (*domain.Project, error) {
	return s.db.FindProject(id)
}

func (s *ProjectService) Delete(ctx context.Context, id int64) error {
	project, err := s.db.FindProject(id)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}

	if project.ContainerID != "" && s.docker != nil {
		if err := s.docker.StopContainer(ctx, project.ContainerID); err != nil {
			slog.Warn("stop container error", "container_id", project.ContainerID, "error", err)
		}
		if err := s.docker.RemoveContainer(ctx, project.ContainerID); err != nil {
			slog.Warn("remove container error", "container_id", project.ContainerID, "error", err)
		}
	}

	if err := s.traefik.RemoveRoute(project.ID); err != nil {
		slog.Warn("traefik remove route error", "project_id", project.ID, "domain", project.Domain, "error", err)
	}

	if err := s.db.DeleteDeploymentsByProject(id); err != nil {
		slog.Warn("delete deployments error", "project_id", id, "error", err)
	}
	if err := s.db.DeleteEnvVarsByProject(id); err != nil {
		slog.Warn("delete env vars error", "project_id", id, "error", err)
	}

	if err := s.db.DeleteProject(id); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	workDir := filepath.Join(s.cfg.DataDir, "projects", fmt.Sprintf("%d", id))
	os.RemoveAll(workDir)

	return nil
}

// Restart recreates the project's container (stop, remove, re-create,
// start) rather than a plain stop+start. This is intentional, not an
// accident: Docker has no way to inject env var changes into an
// already-running container, so recreating from the current DB state on
// every restart is the only way an env var added/changed after the
// container was first created can ever actually take effect (BUG-021).
// The container is re-created from the same already-built image tag, so
// no rebuild/reclone happens here - this is not a full redeploy.
func (s *ProjectService) Restart(ctx context.Context, id int64) error {
	project, err := s.db.FindProject(id)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	if project.ContainerID == "" {
		return fmt.Errorf("no container to restart")
	}
	if err := s.requireDocker(); err != nil {
		return err
	}

	envVars, err := s.db.ListEnvVars(id)
	if err != nil {
		return fmt.Errorf("list env vars: %w", err)
	}

	containerName := fmt.Sprintf("project-%d", project.ID)
	tag := fmt.Sprintf("tamga-project-%d", project.ID)

	if err := s.docker.StopContainer(ctx, project.ContainerID); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}
	if err := s.docker.RemoveContainer(ctx, project.ContainerID); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}

	if err := s.docker.EnsureNetwork(ctx, "tamga-net", false); err != nil {
		return fmt.Errorf("ensure project network: %w", err)
	}

	containerID, err := s.docker.CreateContainer(ctx, containerName, tag, envVarsToSlice(envVars), "tamga-net")
	if err != nil {
		return fmt.Errorf("create container: %w", err)
	}
	if err := s.docker.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("start container: %w", err)
	}

	project.ContainerID = containerID
	if err := s.db.UpdateProject(project); err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	slog.Info("container recreated on restart", "project_id", project.ID, "container_id", containerID[:12])

	return nil
}

// envVarsToSlice converts the project's stored env vars into the
// "KEY=VALUE" string slice format Docker's client expects
// (docker/client.go's CreateContainer(Opts)).
func envVarsToSlice(vars []*domain.EnvVar) []string {
	if len(vars) == 0 {
		return nil
	}
	env := make([]string, 0, len(vars))
	for _, v := range vars {
		env = append(env, fmt.Sprintf("%s=%s", v.Key, v.Value))
	}
	return env
}

type UpdateProjectRequest struct {
	Name       *string            `json:"name,omitempty"`
	SourceType *domain.SourceType `json:"source_type,omitempty"`
	RepoURL    *string            `json:"repo_url,omitempty"`
	Domain     *string            `json:"domain,omitempty"`
	Branch     *string            `json:"branch,omitempty"`
}

func (s *ProjectService) Update(ctx context.Context, id int64, req UpdateProjectRequest) (*domain.Project, error) {
	project, err := s.db.FindProject(id)
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	oldDomain := project.Domain

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
	if err := s.db.UpdateProject(project); err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}

	// Move the Traefik route when a deployed project's domain actually
	// changes - the gap TEST-010 found: the old Caddy-based code never
	// touched routing here at all, leaving the old domain's route
	// dangling and the new domain unrouted until a backend restart. Since
	// each project's route file is keyed by project ID, not domain,
	// "moving" the route is just overwriting project-<id>.yml with the
	// new Host() rule - no separate remove-old step needed unless the
	// domain was cleared entirely.
	if req.Domain != nil && project.Domain != oldDomain && project.ContainerID != "" && s.docker != nil {
		if project.Domain == "" {
			if err := s.traefik.RemoveRoute(project.ID); err != nil {
				slog.Warn("traefik remove route on domain change", "project_id", project.ID, "error", err)
			}
		} else {
			containerName := fmt.Sprintf("project-%d", project.ID)
			port, err := s.docker.GetContainerPort(ctx, project.ContainerID)
			if err != nil {
				port = "80"
			}
			upstream := fmt.Sprintf("%s:%s", containerName, port)
			if err := s.traefik.AddRoute(project.ID, project.Domain, upstream); err != nil {
				slog.Warn("traefik update route on domain change", "project_id", project.ID, "domain", project.Domain, "error", err)
			}
		}
	}

	return project, nil
}

func (s *ProjectService) GetDeployments(ctx context.Context, id int64) ([]*domain.Deployment, error) {
	return s.db.ListDeployments(id)
}

func (s *ProjectService) ListEnvVars(ctx context.Context, projectID int64) ([]*domain.EnvVar, error) {
	return s.db.ListEnvVars(projectID)
}

func (s *ProjectService) CreateEnvVar(ctx context.Context, projectID int64, key, value string) (*domain.EnvVar, error) {
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

func (s *ProjectService) DeleteEnvVar(ctx context.Context, id int64) error {
	return s.db.DeleteEnvVar(id)
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
