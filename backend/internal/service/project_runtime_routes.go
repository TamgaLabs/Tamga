package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	traefikrepo "github.com/TamgaLabs/Tamga/backend/internal/repository/traefik"
)

type projectRuntimeContainer struct {
	Name    string
	Running bool
}

type projectRuntime interface {
	InspectContainer(context.Context, string) (projectRuntimeContainer, error)
}

type projectRoutePublisher interface {
	ReplaceRoutes(int64, []traefikrepo.Route) error
	RemoveRoute(int64) error
}

type dockerProjectRuntime struct{ client *dockerclient.Client }

func (r dockerProjectRuntime) InspectContainer(ctx context.Context, id string) (projectRuntimeContainer, error) {
	info, err := r.client.InspectContainer(ctx, id)
	if err != nil {
		return projectRuntimeContainer{}, err
	}
	name := ""
	if info.Name != "" {
		name = info.Name[1:]
	}
	return projectRuntimeContainer{Name: name, Running: info.State != nil && info.State.Running}, nil
}

// ReconcileProjectRoutes atomically publishes every persisted canonical route
// belonging to one running project, withdrawing stale routes on any failed
// runtime identity lookup.
func (s *SealService) ReconcileProjectRoutes(ctx context.Context, sealID, projectID int64) error {
	if s.routes == nil {
		return nil
	}
	project, err := s.db.FindProject(sealID, projectID)
	if err != nil {
		return fmt.Errorf("find project: %w", err)
	}
	if project.Status != domain.ProjectStatusRunning || s.runtime == nil {
		return s.withdrawProjectRoutes(projectID)
	}
	services, err := s.db.ListServices(sealID, projectID)
	if err != nil {
		return fmt.Errorf("list project services: %w", err)
	}
	containers, err := s.db.ListServiceContainers(projectID)
	if err != nil {
		return fmt.Errorf("list project service containers: %w", err)
	}
	byServiceID := make(map[int64]*domain.ServiceContainer, len(containers))
	for _, container := range containers {
		byServiceID[container.ServiceID] = container
	}
	output := make([]traefikrepo.Route, 0)
	for _, service := range services {
		routes, err := s.db.ListServiceRoutes(sealID, projectID, service.ID)
		if err != nil {
			return fmt.Errorf("list project service routes: %w", err)
		}
		if len(routes) == 0 {
			continue
		}
		container := byServiceID[service.ID]
		if container == nil {
			return s.withdrawProjectRoutes(projectID)
		}
		actual, err := s.runtime.InspectContainer(ctx, container.ContainerID)
		if err != nil || !actual.Running || actual.Name == "" {
			return s.withdrawProjectRoutes(projectID)
		}
		for _, route := range routes {
			output = append(output, traefikrepo.Route{Service: service.Name, Domain: route.Domain, Upstream: fmt.Sprintf("%s:%d", actual.Name, service.InternalPort)})
		}
	}
	if len(output) == 0 {
		return s.withdrawProjectRoutes(projectID)
	}
	if err := s.routes.ReplaceRoutes(projectID, output); err != nil {
		return fmt.Errorf("write project routes: %w", err)
	}
	return nil
}

func (s *SealService) ReconcileRuntime(ctx context.Context) {
	if s.runtime == nil || s.routes == nil {
		return
	}
	seals, err := s.db.ListSeals()
	if err != nil {
		slog.Warn("reconcile project runtime: list seals", "error", err)
		return
	}
	for _, seal := range seals {
		projects, err := s.db.ListProjects(seal.ID)
		if err != nil {
			slog.Warn("reconcile project runtime: list projects", "seal_id", seal.ID, "error", err)
			continue
		}
		for _, project := range projects {
			if err := s.ReconcileProjectRoutes(ctx, seal.ID, project.ID); err != nil {
				slog.Warn("reconcile project routes", "seal_id", seal.ID, "project_id", project.ID, "error", err)
			}
		}
	}
}

func (s *SealService) withdrawProjectRoutes(projectID int64) error {
	if err := s.routes.RemoveRoute(projectID); err != nil {
		return fmt.Errorf("withdraw project routes: %w", err)
	}
	return nil
}
