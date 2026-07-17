package service_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestSealServiceRoutesNormalizeAndPreserveOnConflict(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "tamga.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	svc := service.NewSealService(db, config.Config{DataDir: t.TempDir()})
	seal, err := svc.Create(t.Context(), service.CreateSealRequest{Name: "routes"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}
	repository := &domain.SealRepository{SealID: seal.ID, DisplayName: "app", RemoteURL: "https://example.test/app.git", Branch: "main", WorkspacePath: "repositories/app", Status: domain.ProjectSourceStatusReady}
	if err := db.CreateSealRepository(repository); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	web := &domain.SealService{SealID: seal.ID, RepositoryID: repository.ID, Name: "web", BuildContext: ".", InternalPort: 3000}
	api := &domain.SealService{SealID: seal.ID, RepositoryID: repository.ID, Name: "api", BuildContext: ".", InternalPort: 8080}
	if err := db.CreateSealService(web); err != nil {
		t.Fatalf("create web service: %v", err)
	}
	if err := db.CreateSealService(api); err != nil {
		t.Fatalf("create api service: %v", err)
	}

	first, err := svc.AddServiceRoute(t.Context(), seal.ID, web.ID, service.CreateSealServiceRouteRequest{Domain: " App.Example.Test "})
	if err != nil || first.Domain != "app.example.test" {
		t.Fatalf("add normalized route = %+v, err=%v", first, err)
	}
	if _, err := svc.AddServiceRoute(t.Context(), seal.ID, web.ID, service.CreateSealServiceRouteRequest{Domain: "www.example.test"}); err != nil {
		t.Fatalf("add second route: %v", err)
	}
	if _, err := svc.AddServiceRoute(t.Context(), seal.ID, api.ID, service.CreateSealServiceRouteRequest{Domain: "APP.EXAMPLE.TEST"}); !errors.Is(err, service.ErrSealServiceRouteConflict) {
		t.Fatalf("case-insensitive conflict error = %v", err)
	}

	routes, err := svc.ListServiceRoutes(t.Context(), seal.ID, web.ID)
	if err != nil || len(routes) != 2 {
		t.Fatalf("routes after conflict = %+v, err=%v", routes, err)
	}
	if _, err := svc.ListServiceRoutes(t.Context(), seal.ID, api.ID); err != nil {
		t.Fatalf("empty route set: %v", err)
	}
	if _, err := svc.AddServiceRoute(t.Context(), seal.ID, web.ID, service.CreateSealServiceRouteRequest{Domain: "https://app.example.test"}); !errors.Is(err, service.ErrInvalidSealServiceRoute) {
		t.Fatalf("non-domain route error = %v", err)
	}
}
