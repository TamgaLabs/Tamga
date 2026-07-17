//go:build integration

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
)

// TestProjectServiceDeployStackServiceNameAlias exercises deployStack with a
// real two-service compose fixture. It belongs to the explicit Docker lane
// because it pulls images and creates containers and networks.
func TestProjectServiceDeployStackServiceNameAlias(t *testing.T) {
	docker, err := dockerclient.New()
	if err != nil {
		t.Skipf("docker client not available: %v", err)
	}
	ctx := context.Background()
	if _, err := docker.DockerInfo(ctx); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}

	svc, _ := newTestProjectService(t)
	svc.docker = docker

	deployCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	project := &domain.Project{Name: "alias-test-" + t.Name(), SourceType: domain.SourceTypeRemote}
	if err := svc.db.CreateProject(project); err != nil {
		t.Fatalf("create project row: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		netName := sealNetworkName(project.ID)
		for _, service := range []string{"web", "redis"} {
			name := serviceContainerName(project.ID, service)
			docker.StopContainer(cleanupCtx, name)
			docker.RemoveContainer(cleanupCtx, name)
		}
		docker.NetworkRemove(cleanupCtx, netName)
	})

	services := []domain.ComposeService{
		{Name: "redis", Image: "redis:7-alpine"},
		{Name: "web", Image: "nginx:alpine", DependsOn: []string{"redis"}},
	}
	if err := svc.deployStack(deployCtx, project, services, true); err != nil {
		t.Fatalf("deployStack: %v", err)
	}

	webName := serviceContainerName(project.ID, "web")
	execID, err := docker.ExecCreate(deployCtx, webName, []string{"getent", "hosts", "redis"}, "")
	if err != nil {
		t.Fatalf("ExecCreate: %v", err)
	}
	hijacked, err := docker.ExecAttach(deployCtx, execID)
	if err != nil {
		t.Fatalf("ExecAttach: %v", err)
	}
	defer hijacked.Close()

	buf := make([]byte, 4096)
	n, _ := hijacked.Reader.Read(buf)
	if output := string(buf[:n]); !strings.Contains(output, "redis") {
		t.Errorf("bare service-name alias %q did not resolve from peer web container; getent output: %q", "redis", output)
	}
}
