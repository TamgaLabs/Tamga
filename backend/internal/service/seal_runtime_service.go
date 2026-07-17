package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
)

// sealRuntime is deliberately smaller than the Docker client. It captures the
// Seal-owned lifecycle operations and keeps the service testable without a
// daemon; dockerSealRuntime is the only production adapter.
type sealRuntime interface {
	EnsureNetwork(context.Context, string, bool) error
	ContainerExists(context.Context, string) bool
	RemoveContainer(context.Context, string) error
	CreateContainer(context.Context, string, string, []string, string, []string, []string) (string, error)
	StartContainer(context.Context, string) error
	InspectContainer(context.Context, string) (sealRuntimeContainer, error)
}

type sealRuntimeContainer struct {
	ID      string
	Name    string
	Running bool
}

type dockerSealRuntime struct{ client *dockerclient.Client }

func (r dockerSealRuntime) EnsureNetwork(ctx context.Context, name string, internal bool) error {
	return r.client.EnsureNetwork(ctx, name, internal)
}

func (r dockerSealRuntime) ContainerExists(ctx context.Context, name string) bool {
	return r.client.ContainerExists(ctx, name)
}

func (r dockerSealRuntime) RemoveContainer(ctx context.Context, name string) error {
	return r.client.RemoveContainer(ctx, name)
}

func (r dockerSealRuntime) CreateContainer(ctx context.Context, name, image string, env []string, network string, mounts, aliases []string) (string, error) {
	return r.client.CreateContainerOpts(ctx, name, image, env, network, mounts, container.Resources{}, false, aliases)
}

func (r dockerSealRuntime) StartContainer(ctx context.Context, id string) error {
	return r.client.StartContainer(ctx, id)
}

func (r dockerSealRuntime) InspectContainer(ctx context.Context, id string) (sealRuntimeContainer, error) {
	info, err := r.client.InspectContainer(ctx, id)
	if err != nil {
		return sealRuntimeContainer{}, err
	}
	name := ""
	if info.Name != "" {
		name = info.Name[1:]
	}
	return sealRuntimeContainer{ID: info.ID, Name: name, Running: info.State != nil && info.State.Running}, nil
}

func (s *SealService) requireRuntime() error {
	if s.runtime == nil {
		return fmt.Errorf("docker daemon not available")
	}
	return nil
}

// Deploy creates the private runtime described by a direct, image-based Seal
// Compose configuration. Container identity is persisted only from Docker's
// CreateContainer result, never reconstructed from a naming convention.
// Generated configurations have build declarations and must first acquire a
// Seal-native build lifecycle; treating them as image-based deployments would
// silently deploy an empty image reference, so this method rejects them.
func (s *SealService) Deploy(ctx context.Context, sealID int64) error {
	if err := s.requireRuntime(); err != nil {
		return err
	}
	seal, err := s.db.FindSeal(sealID)
	if err != nil {
		return fmt.Errorf("find seal: %w", err)
	}
	if seal.ConfigAuthority != configurationAuthorityDirect {
		return fmt.Errorf("generated Seal configuration must be built before deployment")
	}
	services, err := ParseComposeYAML(seal.ComposeYAML)
	if err != nil {
		return fmt.Errorf("parse Seal compose: %w", err)
	}
	if err := s.deployRuntime(ctx, seal, services); err != nil {
		seal.Status = domain.ProjectStatusError
		_ = s.db.UpdateSeal(seal)
		return err
	}
	return nil
}

func (s *SealService) deployRuntime(ctx context.Context, seal *domain.Seal, services []domain.ComposeService) error {
	network := sealNetworkName(seal.ID)
	if err := s.runtime.EnsureNetwork(ctx, network, true); err != nil {
		return fmt.Errorf("ensure Seal internal network: %w", err)
	}
	order, err := TopoSortServices(toComposeServiceDeps(services))
	if err != nil {
		return fmt.Errorf("order Seal services: %w", err)
	}
	byName := make(map[string]domain.ComposeService, len(services))
	for _, service := range services {
		byName[service.Name] = service
	}
	containers := make([]*domain.ServiceContainer, 0, len(services))
	for _, name := range order {
		service := byName[name]
		if service.Image == "" {
			return fmt.Errorf("Seal service %q requires an image", service.Name)
		}
		containerName := serviceContainerName(seal.ID, service.Name)
		if s.runtime.ContainerExists(ctx, containerName) {
			if err := s.runtime.RemoveContainer(ctx, containerName); err != nil {
				return fmt.Errorf("replace Seal service %q: %w", service.Name, err)
			}
		}
		containerID, err := s.runtime.CreateContainer(ctx, containerName, service.Image, envMapToSlice(service.Environment), network, composeVolumesToMounts(service.Volumes), []string{service.Name})
		if err != nil {
			return fmt.Errorf("create Seal service %q: %w", service.Name, err)
		}
		if err := s.runtime.StartContainer(ctx, containerID); err != nil {
			return fmt.Errorf("start Seal service %q: %w", service.Name, err)
		}
		actual, err := s.runtime.InspectContainer(ctx, containerID)
		if err != nil {
			return fmt.Errorf("verify Seal service %q identity: %w", service.Name, err)
		}
		if !actual.Running || actual.ID == "" || actual.Name == "" {
			return fmt.Errorf("verify Seal service %q identity: container is not running", service.Name)
		}
		containers = append(containers, &domain.ServiceContainer{ProjectID: seal.ID, ServiceName: service.Name, ContainerID: actual.ID, ContainerName: actual.Name, Status: "running"})
	}
	if err := s.db.ReplaceServiceContainers(seal.ID, containers); err != nil {
		return fmt.Errorf("record Seal service containers: %w", err)
	}
	seal.Status = domain.ProjectStatusRunning
	seal.ContainerID = containers[0].ContainerID
	if err := s.db.UpdateSeal(seal); err != nil {
		return fmt.Errorf("record running Seal: %w", err)
	}
	return nil
}

// ReconcileRuntime recovers persisted service identity from Docker on API
// startup. It never publishes Traefik routes; route reconciliation remains a
// separate future concern. Missing or stopped services mark only their owning
// Seal as error and leave its persisted identity intact for diagnosis.
func (s *SealService) ReconcileRuntime(ctx context.Context) {
	if s.runtime == nil {
		return
	}
	seals, err := s.db.ListSeals()
	if err != nil {
		slog.Warn("reconcile Seal runtime: list Seals failed", "error", err)
		return
	}
	for _, seal := range seals {
		if seal.Status != domain.ProjectStatusRunning {
			continue
		}
		containers, err := s.db.ListServiceContainers(seal.ID)
		if err != nil || len(containers) == 0 {
			s.markSealRuntimeError(seal, "no persisted service containers", err)
			continue
		}
		refreshed := make([]*domain.ServiceContainer, 0, len(containers))
		allRunning := true
		for _, persisted := range containers {
			actual, inspectErr := s.runtime.InspectContainer(ctx, persisted.ContainerID)
			if inspectErr != nil || !actual.Running || actual.ID == "" || actual.Name == "" {
				allRunning = false
				break
			}
			refreshed = append(refreshed, &domain.ServiceContainer{ProjectID: seal.ID, ServiceName: persisted.ServiceName, ContainerID: actual.ID, ContainerName: actual.Name, Status: "running"})
		}
		if !allRunning {
			s.markSealRuntimeError(seal, "a persisted service is not running", nil)
			continue
		}
		if err := s.db.ReplaceServiceContainers(seal.ID, refreshed); err != nil {
			s.markSealRuntimeError(seal, "persist recovered container identity", err)
			continue
		}
		seal.ContainerID = refreshed[0].ContainerID
		if err := s.db.UpdateSeal(seal); err != nil {
			slog.Warn("reconcile Seal runtime: update Seal", "seal_id", seal.ID, "error", err)
		}
	}
}

func (s *SealService) markSealRuntimeError(seal *domain.Seal, reason string, err error) {
	seal.Status = domain.ProjectStatusError
	if updateErr := s.db.UpdateSeal(seal); updateErr != nil {
		slog.Warn("reconcile Seal runtime: "+reason, "seal_id", seal.ID, "error", err, "update_error", updateErr)
		return
	}
	slog.Warn("reconcile Seal runtime: "+reason, "seal_id", seal.ID, "error", err)
}

// RunningServiceTarget gives the later route publisher a verified internal
// DNS target. It uses the declared internal port and Docker-confirmed
// container name rather than guessing either value from a Seal ID.
func (s *SealService) RunningServiceTarget(ctx context.Context, sealID, serviceID int64) (string, error) {
	if err := s.requireRuntime(); err != nil {
		return "", err
	}
	service, err := s.db.FindSealService(sealID, serviceID)
	if err != nil {
		return "", fmt.Errorf("find Seal service: %w", err)
	}
	containers, err := s.db.ListServiceContainers(sealID)
	if err != nil {
		return "", fmt.Errorf("list Seal service containers: %w", err)
	}
	for _, persisted := range containers {
		if persisted.ServiceName != service.Name {
			continue
		}
		actual, err := s.runtime.InspectContainer(ctx, persisted.ContainerID)
		if err != nil || !actual.Running || actual.Name == "" {
			return "", fmt.Errorf("Seal service %q is not running", service.Name)
		}
		return fmt.Sprintf("%s:%d", actual.Name, service.InternalPort), nil
	}
	return "", fmt.Errorf("Seal service %q has no persisted container identity", service.Name)
}
