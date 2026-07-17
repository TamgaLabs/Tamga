package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

type fakeSealRuntime struct {
	containers map[string]sealRuntimeContainer
	networks   []string
}

func (r *fakeSealRuntime) EnsureNetwork(_ context.Context, name string, internal bool) error {
	if !internal {
		return fmt.Errorf("network must be internal")
	}
	r.networks = append(r.networks, name)
	return nil
}
func (r *fakeSealRuntime) ContainerExists(_ context.Context, name string) bool {
	_, ok := r.containers[name]
	return ok
}
func (r *fakeSealRuntime) RemoveContainer(_ context.Context, name string) error {
	delete(r.containers, name)
	return nil
}
func (r *fakeSealRuntime) CreateContainer(_ context.Context, name, _ string, _ []string, _ string, _ []string, _ []string) (string, error) {
	id := "actual-" + name
	r.containers[id] = sealRuntimeContainer{ID: id, Name: name}
	return id, nil
}
func (r *fakeSealRuntime) StartContainer(_ context.Context, id string) error {
	c := r.containers[id]
	c.Running = true
	r.containers[id] = c
	return nil
}
func (r *fakeSealRuntime) InspectContainer(_ context.Context, id string) (sealRuntimeContainer, error) {
	c, ok := r.containers[id]
	if !ok {
		return sealRuntimeContainer{}, fmt.Errorf("not found")
	}
	return c, nil
}

func newSealRuntimeTestService(t *testing.T) (*SealService, *sqlite.DB, *fakeSealRuntime) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "seals.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	runtime := &fakeSealRuntime{containers: map[string]sealRuntimeContainer{}}
	return &SealService{db: db, cfg: config.Config{DataDir: t.TempDir()}, runtime: runtime}, db, runtime
}

func TestSealRuntimeDeployPersistsActualContainerIdentityAndTarget(t *testing.T) {
	svc, db, runtime := newSealRuntimeTestService(t)
	seal := &domain.Seal{Name: "runtime", SourceType: domain.SourceTypeCompose, Status: domain.ProjectStatusConfiguring, ConfigAuthority: configurationAuthorityDirect, ComposeYAML: "services:\n  web:\n    image: nginx:alpine\n    ports: [\"8080\"]\n"}
	if err := db.CreateSeal(seal); err != nil {
		t.Fatal(err)
	}
	repository := &domain.SealRepository{SealID: seal.ID, DisplayName: "web", RemoteURL: "https://example.invalid/web.git", Branch: "main", WorkspacePath: "repositories/web", Status: domain.ProjectSourceStatusReady}
	if err := db.CreateSealRepository(repository); err != nil {
		t.Fatal(err)
	}
	service := &domain.SealService{SealID: seal.ID, RepositoryID: repository.ID, Name: "web", BuildContext: ".", InternalPort: 8080}
	if err := db.CreateSealService(service); err != nil {
		t.Fatal(err)
	}
	if err := svc.Deploy(context.Background(), seal.ID); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if len(runtime.networks) != 1 || runtime.networks[0] != sealNetworkName(seal.ID) {
		t.Fatalf("internal network = %v", runtime.networks)
	}
	containers, err := db.ListServiceContainers(seal.ID)
	if err != nil || len(containers) != 1 {
		t.Fatalf("persisted containers = %+v, err=%v", containers, err)
	}
	if containers[0].ContainerID != "actual-seal-1-web" || containers[0].ContainerName != "seal-1-web" {
		t.Fatalf("container identity was not derived from runtime: %+v", containers[0])
	}
	target, err := svc.RunningServiceTarget(context.Background(), seal.ID, service.ID)
	if err != nil || target != "seal-1-web:8080" {
		t.Fatalf("running target = %q, err=%v", target, err)
	}
}

func TestSealRuntimeReconcileMarksMissingContainerError(t *testing.T) {
	svc, db, _ := newSealRuntimeTestService(t)
	seal := &domain.Seal{Name: "stale", SourceType: domain.SourceTypeCompose, Status: domain.ProjectStatusRunning, ConfigAuthority: configurationAuthorityDirect}
	if err := db.CreateSeal(seal); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceServiceContainers(seal.ID, []*domain.ServiceContainer{{ProjectID: seal.ID, ServiceName: "web", ContainerID: "missing", ContainerName: "seal-1-web", Status: "running"}}); err != nil {
		t.Fatal(err)
	}
	svc.ReconcileRuntime(context.Background())
	recovered, err := db.FindSeal(seal.ID)
	if err != nil || recovered.Status != domain.ProjectStatusError {
		t.Fatalf("reconciled Seal = %+v, err=%v", recovered, err)
	}
}
