package deployments

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/TamgaLabs/Tamga/internal/database"
	dockerclient "github.com/TamgaLabs/Tamga/internal/docker"
	"github.com/TamgaLabs/Tamga/internal/git"
	"github.com/TamgaLabs/Tamga/internal/proxy"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries *database.Queries
	docker  *dockerclient.Client
	cloner  *git.Cloner
	network string
}

func NewService(queries *database.Queries, docker *dockerclient.Client, workDir, network string) *Service {
	return &Service{
		queries: queries,
		docker:  docker,
		cloner:  git.NewCloner(workDir),
		network: network,
	}
}

type DeployParams struct {
	DeploymentID pgtype.UUID
	ProjectID    pgtype.UUID
	UserID       pgtype.UUID
}

func (s *Service) Deploy(ctx context.Context, p DeployParams) {
	log.Printf("deployment %s: starting", p.DeploymentID.String())

	s.logLine(ctx, p.DeploymentID, "stdout", "Starting deployment...")

	s.updateStatus(ctx, p.DeploymentID, "building")

	repo, err := s.queries.GetGitRepositoryByProject(ctx, database.GetGitRepositoryByProjectParams{
		ProjectID: p.ProjectID,
		UserID:    p.UserID,
	})
	if err != nil {
		s.fail(ctx, p.DeploymentID, "No git repository connected")
		return
	}

	s.logLine(ctx, p.DeploymentID, "stdout", fmt.Sprintf("Cloning %s (branch: %s)...", repo.Url, repo.Branch))

	cloneResult, err := s.cloner.Clone(ctx, repo.Url, repo.Branch)
	if err != nil {
		s.fail(ctx, p.DeploymentID, "Clone failed: "+err.Error())
		return
	}
	defer s.cloner.Cleanup(cloneResult.Path)

	s.logLine(ctx, p.DeploymentID, "stdout", fmt.Sprintf("Commit %s: %s", cloneResult.Commit, cloneResult.Message))

	files, err := s.cloner.ListFiles(cloneResult.Path)
	if err != nil {
		s.fail(ctx, p.DeploymentID, "Failed to read repo: "+err.Error())
		return
	}

	runtime := dockerclient.DetectRuntime(files)
	imageTag := fmt.Sprintf("tamga-%s:%s", shortID(p.ProjectID), shortCloneID(cloneResult.Commit))

	var dockerfileContent string
	if runtime == "dockerfile" {
		s.logLine(ctx, p.DeploymentID, "stdout", "Using Dockerfile from repository")
		dfPath := git.DockerfilePath(cloneResult.Path)
		content, err := git.ReadFile(dfPath)
		if err != nil {
			s.fail(ctx, p.DeploymentID, "Failed to read Dockerfile: "+err.Error())
			return
		}
		dockerfileContent = content
	} else {
		s.logLine(ctx, p.DeploymentID, "stdout", fmt.Sprintf("No Dockerfile found, detected: %s", runtime))
		dockerfileContent = dockerclient.GenerateDockerfile(runtime)
		if dockerfileContent == "" {
			s.fail(ctx, p.DeploymentID, "Unsupported runtime: "+runtime)
			return
		}
	}

	s.logLine(ctx, p.DeploymentID, "stdout", "Building image...")
	buildCtx, err := dockerclient.NewBuildContext(dockerfileContent, nil)
	if err != nil {
		s.fail(ctx, p.DeploymentID, "Build context error: "+err.Error())
		return
	}

	logWriter := &buildLogWriter{fn: func(line string) {
		s.logLine(ctx, p.DeploymentID, "stdout", line)
	}}

	if err := s.docker.BuildImage(ctx, dockerclient.BuildOptions{
		BuildContext: buildCtx,
		Tag:          imageTag,
		Labels:       map[string]string{"tamga.project": p.ProjectID.String()},
	}, logWriter); err != nil {
		s.fail(ctx, p.DeploymentID, "Build failed: "+err.Error())
		return
	}
	s.logLine(ctx, p.DeploymentID, "stdout", "Image built: "+imageTag)

	domains, err := s.queries.ListDomainsByProject(ctx, database.ListDomainsByProjectParams{
		ProjectID: p.ProjectID,
		UserID:    p.UserID,
	})
	if err != nil {
		s.fail(ctx, p.DeploymentID, "Failed to list domains")
		return
	}

	envVars, err := s.queries.ListEnvVarsByProject(ctx, database.ListEnvVarsByProjectParams{
		ProjectID: p.ProjectID,
		UserID:    p.UserID,
	})
	if err != nil {
		s.fail(ctx, p.DeploymentID, "Failed to list env vars")
		return
	}

	s.logLine(ctx, p.DeploymentID, "stdout", "Stopping previous containers...")
	s.stopExistingContainers(ctx, p.ProjectID)

	domain := ""
	if len(domains) > 0 {
		domain = domains[0].Domain
	}

	containerName := fmt.Sprintf("tamga-%s", shortID(p.ProjectID))
	internalPort := runtimePort(runtime)
	env := buildEnv(envVars, internalPort)

	traefikLabels := proxy.Labels{
		"tamga.project":    p.ProjectID.String(),
		"tamga.deployment": p.DeploymentID.String(),
	}
	if domain != "" {
		for k, v := range proxy.GenerateLabels(proxy.TraefikConfig{
			Domain:       domain,
			InternalPort: internalPort,
			ProjectName:  p.ProjectID.String(),
		}) {
			traefikLabels[k] = v
		}
	}

	s.logLine(ctx, p.DeploymentID, "stdout", "Creating container...")
	containerID, err := s.docker.CreateContainer(ctx,
		&container.Config{
			Image:  imageTag,
			Env:    env,
			Labels: traefikLabels,
		},
		&container.HostConfig{
			NetworkMode: container.NetworkMode(s.network),
		},
		&network.NetworkingConfig{},
		containerName,
	)
	if err != nil {
		s.fail(ctx, p.DeploymentID, "Container creation failed: "+err.Error())
		return
	}

	s.logLine(ctx, p.DeploymentID, "stdout", "Starting container...")
	if err := s.docker.StartContainer(ctx, containerID); err != nil {
		s.fail(ctx, p.DeploymentID, "Failed to start: "+err.Error())
		return
	}

	s.logLine(ctx, p.DeploymentID, "stdout", fmt.Sprintf("Container started: %s", containerID[:12]))

	s.queries.UpdateDeploymentDetails(ctx, database.UpdateDeploymentDetailsParams{
		ID:            p.DeploymentID,
		Status:        "running",
		CommitSha:     cloneResult.Commit,
		CommitMessage: cloneResult.Message,
		ImageTag:      imageTag,
		ContainerID:   containerID,
		Domain:        domain,
	})

	s.logLine(ctx, p.DeploymentID, "stdout", "Deployment complete!")
	if domain != "" {
		s.logLine(ctx, p.DeploymentID, "stdout", fmt.Sprintf("App is live at: https://%s", domain))
	}
}

func (s *Service) stopExistingContainers(ctx context.Context, projectID pgtype.UUID) {
	containers, err := s.docker.ListContainers(ctx)
	if err != nil {
		log.Printf("failed to list containers: %v", err)
		return
	}
	pid := projectID.String()
	for _, c := range containers {
		if c.Labels["tamga.project"] == pid {
			timeout := 5 * time.Second
			s.docker.StopContainer(ctx, c.ID, &timeout)
			s.docker.RemoveContainer(ctx, c.ID, true)
		}
	}
}

func (s *Service) updateStatus(ctx context.Context, id pgtype.UUID, status string) {
	if _, err := s.queries.UpdateDeploymentStatus(ctx, database.UpdateDeploymentStatusParams{
		ID:     id,
		Status: status,
	}); err != nil {
		log.Printf("failed to update deployment %s status: %v", id.String(), err)
	}
}

func (s *Service) logLine(ctx context.Context, deploymentID pgtype.UUID, stream, message string) {
	for _, line := range strings.Split(message, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			s.queries.CreateDeploymentLog(ctx, database.CreateDeploymentLogParams{
				DeploymentID: deploymentID,
				Stream:       stream,
				Message:      trimmed,
			})
		}
	}
}

func (s *Service) fail(ctx context.Context, deploymentID pgtype.UUID, msg string) {
	log.Printf("deployment %s: %s", deploymentID.String(), msg)
	s.logLine(ctx, deploymentID, "stderr", "ERROR: "+msg)
	s.updateStatus(ctx, deploymentID, "failed")
}

type buildLogWriter struct {
	fn func(string)
}

func (w *buildLogWriter) Write(p []byte) (int, error) {
	w.fn(string(p))
	return len(p), nil
}

func shortID(id pgtype.UUID) string {
	s := id.String()
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func shortCloneID(commit string) string {
	if len(commit) > 8 {
		return commit[:8]
	}
	return commit
}

func runtimePort(runtime string) string {
	switch runtime {
	case "node":
		return "3000"
	case "go":
		return "8080"
	case "python":
		return "8000"
	default:
		return "3000"
	}
}

func buildEnv(envVars []database.EnvVar, port string) []string {
	env := make([]string, 0, len(envVars)+2)
	env = append(env, "PORT="+port)
	for _, e := range envVars {
		env = append(env, e.Key+"="+e.Value)
	}
	return env
}
