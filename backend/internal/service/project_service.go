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
	// ComposeYAML, if non-empty, makes this a compose-project create
	// (FEAT-029): the handler has already parsed/validated it (and
	// ExposedService, if set) via ParseComposeYAML before calling Create,
	// so deploy() picks it up and runs FEAT-028's compose branch instead
	// of the git clone+build path.
	ComposeYAML    string `json:"compose_yaml,omitempty"`
	ExposedService string `json:"exposed_service,omitempty"`
}

func (s *ProjectService) Create(ctx context.Context, req CreateProjectRequest) (*domain.Project, error) {
	if req.SourceType == "" {
		req.SourceType = domain.SourceTypeRemote
	}
	if req.Branch == "" {
		req.Branch = "main"
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

		envVars, err := s.db.ListEnvVars(project.ID)
		if err != nil {
			return fmt.Errorf("list env vars: %w", err)
		}
		services = []domain.ComposeService{synthesizeGitBuildService(tag, envVars)}
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
// (verified live in TEST-014). This is only attempted when there is
// actually a route to add (an exposed service resolved AND a domain set)
// - a project with no domain gets no Traefik network wiring at all.
func (s *ProjectService) deployStack(ctx context.Context, project *domain.Project, services []domain.ComposeService, pullImages bool) error {
	netName := projectNetworkName(project.ID)
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

		if extra := extraNetworks(netName, svc.Networks); len(extra) > 0 {
			for _, n := range extra {
				if err := s.docker.EnsureNetwork(ctx, n, false); err != nil {
					slog.Warn("ensure extra service network", "project_id", project.ID, "service", svc.Name, "network", n, "error", err)
				}
			}
			if err := s.docker.ConnectNetworks(ctx, containerName, extra, alias); err != nil {
				slog.Warn("connect service to extra networks", "project_id", project.ID, "service", svc.Name, "error", err)
			}
		}

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

	// Resolve the exposed service and persist that resolution back onto
	// project.ExposedService - whether it came from an explicit override
	// or the heuristic - so it becomes the durable source of truth for
	// ReconcileRoutes/Update/future redeploys, instead of every consumer
	// re-running the heuristic against re-derived compose data.
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

	if hasExposed && project.Domain != "" {
		exposedContainerName := serviceContainerName(project.ID, exposedName)
		port := exposedTargetPort(byName[exposedName])
		if port == "" {
			resolvedPort, perr := s.docker.GetContainerPort(ctx, project.ContainerID)
			if perr != nil {
				resolvedPort = "80"
			}
			port = resolvedPort
		}
		upstream := fmt.Sprintf("%s:%s", exposedContainerName, port)

		s.connectTraefikToNetwork(ctx, netName)

		if err := s.traefik.AddRoute(project.ID, project.Domain, upstream); err != nil {
			slog.Warn("traefik route failed", "project_id", project.ID, "domain", project.Domain, "error", err)
			// non-fatal: containers are running, route can be added manually
		} else {
			slog.Info("traefik route added", "project_id", project.ID, "domain", project.Domain, "upstream", upstream)
		}
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

// ReconcileRoutes re-writes every currently-running project's Traefik
// route (and re-attaches Traefik to that project's network) on backend
// startup - a defensive re-write against drift, not a Caddy-style
// restore-after-wipe (Traefik's file-provider routes already persist on
// disk across backend restarts by themselves; see main.go's call site).
//
// Unlike the pre-FEAT-028 version, the upstream is derived via
// resolveExposedUpstream (project.ExposedService's own persisted
// container row) rather than assuming a single "project-<id>" container -
// a project can now have N service containers named
// "project-<id>-<service>".
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
		if p.Status != domain.ProjectStatusRunning || p.Domain == "" {
			continue
		}
		upstream, ok := s.resolveExposedUpstream(ctx, p)
		if !ok {
			continue
		}

		s.connectTraefikToNetwork(ctx, projectNetworkName(p.ID))

		if err := s.traefik.AddRoute(p.ID, p.Domain, upstream); err != nil {
			slog.Warn("reconcile project route", "project_id", p.ID, "domain", p.Domain, "error", err)
		} else {
			slog.Info("reconciled project route", "project_id", p.ID, "domain", p.Domain, "upstream", upstream)
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

// Delete tears down a project's whole stack: every persisted service
// container (project_service_containers, FEAT-025), the per-project
// network, the Traefik route, then the child DB rows and the project row
// itself. Explicitly calls DeleteServiceContainersByProject rather than
// relying on the schema's ON DELETE CASCADE - this codebase does not
// enable PRAGMA foreign_keys (FEAT-025's finding), so cascade deletes
// never actually fire; DeleteEnvVarsByProject below is the existing
// precedent for the same reason.
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

	if err := s.db.DeleteProject(id); err != nil {
		return fmt.Errorf("delete project: %w", err)
	}

	workDir := filepath.Join(s.cfg.DataDir, "projects", fmt.Sprintf("%d", id))
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
	netName := projectNetworkName(id)
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
	oldDomain := project.Domain
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
			// Project is running - check if the new service can be routed
			if _, ok := s.resolveExposedUpstream(ctx, project); !ok {
				return nil, fmt.Errorf("exposed service %q has no running container to route to", *req.ExposedService)
			}
		}
	}

	if err := s.db.UpdateProject(project); err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}

	// Move the Traefik route when a deployed project's domain or exposed
	// service changes - extends the TEST-010 fix to also rewrite the route
	// when rebinding to a different service (FEAT-040). Since each
	// project's route file is keyed by project ID, not domain, "moving"
	// the route is just overwriting project-<id>.yml with the new Host()
	// rule and the new service's upstream - no separate remove-old step
	// needed unless the domain was cleared entirely. The upstream is
	// resolved via resolveExposedUpstream (project.ExposedService's
	// persisted container row) rather than assuming a single
	// "project-<id>" container - FEAT-028 gives a project N service
	// containers named "project-<id>-<service>" instead.
	domainChanged := project.Domain != oldDomain
	exposedServiceChanged := project.ExposedService != oldExposedService
	if (domainChanged || exposedServiceChanged) && project.ContainerID != "" && s.docker != nil {
		if project.Domain == "" {
			if err := s.traefik.RemoveRoute(project.ID); err != nil {
				slog.Warn("traefik remove route on domain change", "project_id", project.ID, "error", err)
			}
		} else if upstream, ok := s.resolveExposedUpstream(ctx, project); ok {
			if err := s.traefik.AddRoute(project.ID, project.Domain, upstream); err != nil {
				slog.Warn("traefik update route on domain or exposed service change", "project_id", project.ID, "domain", project.Domain, "exposed_service", project.ExposedService, "error", err)
			} else {
				s.connectTraefikToNetwork(ctx, projectNetworkName(project.ID))
			}
		} else {
			slog.Warn("traefik update route on domain or exposed service change: no resolvable exposed service", "project_id", project.ID, "domain", project.Domain, "exposed_service", project.ExposedService)
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
