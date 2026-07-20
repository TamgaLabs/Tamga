package sqlite_test

import (
	"errors"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

func TestCanonicalSealProjectServiceRoutePersistence(t *testing.T) {
	db := openTestDB(t)

	seal := &domain.Seal{Name: "workspace"}
	if err := db.CreateSeal(seal); err != nil {
		t.Fatalf("create seal: %v", err)
	}
	project := &domain.Project{
		SealID:          seal.ID,
		Name:            "application",
		SourceType:      domain.SourceTypeRemote,
		RepoURL:         "https://example.test/application.git",
		Branch:          "main",
		ComposeYAML:     "services: {}\n",
		ConfigAuthority: "generated",
		Status:          domain.ProjectStatusConfiguring,
		ConfigRevision:  2,
		BuildRevision:   1,
	}
	if err := db.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	gotProject, err := db.FindProject(seal.ID, project.ID)
	if err != nil {
		t.Fatalf("find project through seal: %v", err)
	}
	if gotProject.SealID != seal.ID || gotProject.RepoURL != project.RepoURL || gotProject.ConfigRevision != 2 || gotProject.BuildRevision != 1 {
		t.Fatalf("project did not round-trip: %+v", gotProject)
	}

	service := &domain.Service{ProjectID: project.ID, Name: "web", BuildContext: ".", InternalPort: 8080, Dependencies: []string{"worker"}}
	if err := db.CreateService(seal.ID, service); err != nil {
		t.Fatalf("create service through seal: %v", err)
	}
	if _, err := db.FindService(seal.ID, project.ID, service.ID); err != nil {
		t.Fatalf("find owned service: %v", err)
	}
	if _, err := db.FindService(seal.ID+1, project.ID, service.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("find service through wrong seal error = %v, want not found", err)
	}

	route := &domain.ServiceRoute{ServiceID: service.ID, Domain: " App.Example.Test "}
	if err := db.CreateServiceRoute(seal.ID, project.ID, route); err != nil {
		t.Fatalf("create route: %v", err)
	}
	if route.Domain != "app.example.test" {
		t.Fatalf("route domain = %q, want normalized exact domain", route.Domain)
	}
	if err := db.CreateServiceRoute(seal.ID, project.ID, &domain.ServiceRoute{ServiceID: service.ID, Domain: "APP.EXAMPLE.TEST"}); !errors.Is(err, sqlite.ErrServiceRouteDomainConflict) {
		t.Fatalf("duplicate normalized route error = %v, want conflict", err)
	}
	if deleted, err := db.DeleteServiceRoute(seal.ID+1, project.ID, service.ID, route.ID); err != nil || deleted {
		t.Fatalf("delete route through wrong seal = (%v, %v), want (false, nil)", deleted, err)
	}
}
