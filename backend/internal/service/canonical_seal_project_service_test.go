package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

func TestSealProjectServiceOwnsProjectServicesAndRoutes(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "tamga.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	svc := NewSealService(db, config.Config{DataDir: t.TempDir()})
	seal, err := svc.Create(context.Background(), CreateSealRequest{Name: "workspace"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}
	project, err := svc.CreateProject(context.Background(), seal.ID, CreateSealProjectRequest{
		Name: "application", RemoteURL: "https://example.test/application.git", Branch: "main",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if project.SealID != seal.ID || project.Status != domain.ProjectStatusCreated {
		t.Fatalf("created project = %+v", project)
	}
	web, err := svc.CreateProjectService(context.Background(), seal.ID, project.ID, CreateProjectServiceRequest{
		Name: "web", BuildContext: ".", InternalPort: 8080,
	})
	if err != nil {
		t.Fatalf("create project service: %v", err)
	}
	if _, err := svc.CreateProjectService(context.Background(), seal.ID, project.ID, CreateProjectServiceRequest{
		Name: "worker", BuildContext: ".", InternalPort: 9000, Dependencies: []string{"web"},
	}); err != nil {
		t.Fatalf("create dependent service: %v", err)
	}
	route, err := svc.AddProjectServiceRoute(context.Background(), seal.ID, project.ID, web.ID, CreateProjectServiceRouteRequest{Domain: " App.Example.Test "})
	if err != nil || route.Domain != "app.example.test" {
		t.Fatalf("add normalized route = %+v, err=%v", route, err)
	}
	if _, err := svc.AddProjectServiceRoute(context.Background(), seal.ID, project.ID, web.ID, CreateProjectServiceRouteRequest{Domain: "app.example.test"}); !errors.Is(err, ErrProjectServiceRouteConflict) {
		t.Fatalf("duplicate route error = %v", err)
	}

	configuration, err := svc.ProjectConfiguration(context.Background(), seal.ID, project.ID)
	if err != nil {
		t.Fatalf("project configuration: %v", err)
	}
	if configuration.GeneratedCompose == "" || !configuration.BuildPermitted {
		t.Fatalf("generated configuration = %+v", configuration)
	}
}

func TestCreateProjectRedactsRemoteURLCredentials(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "tamga.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}

	svc := NewSealService(db, config.Config{DataDir: t.TempDir()})
	seal, err := svc.Create(context.Background(), CreateSealRequest{Name: "workspace"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}
	project, err := svc.CreateProject(context.Background(), seal.ID, CreateSealProjectRequest{
		Name: "application", RemoteURL: "https://token:secret@example.test/application.git",
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if project.RepoURL != "https://example.test/application.git" {
		t.Fatalf("stored remote URL = %q, want redacted URL", project.RepoURL)
	}
}
