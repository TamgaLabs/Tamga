package service_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestSealServicesAuthorityAndNextJSGeneration(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "seals.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	dataDir := t.TempDir()
	svc := service.NewSealService(db, config.Config{DataDir: dataDir})
	seal, err := svc.Create(context.Background(), service.CreateSealRequest{Name: "next"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}
	repository := &domain.SealRepository{SealID: seal.ID, DisplayName: "web", RemoteURL: "https://github.com/MaxLeiter/maxleiter.com.git", Branch: "main", WorkspacePath: "repositories/web", Status: domain.ProjectSourceStatusReady}
	if err := db.CreateSealRepository(repository); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	checkout := filepath.Join(dataDir, "seals", "1", "repositories", "web")
	if err := os.MkdirAll(checkout, 0755); err != nil {
		t.Fatalf("create checkout: %v", err)
	}
	writePinnedNextJSCheckout(t, checkout)

	web, err := svc.CreateService(context.Background(), seal.ID, service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "web", BuildContext: ".", InternalPort: 3000})
	if err != nil {
		t.Fatalf("create web service: %v", err)
	}
	if _, err := svc.CreateService(context.Background(), seal.ID, service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "web", InternalPort: 3000}); err == nil {
		t.Fatal("duplicate service name was accepted")
	}
	worker, err := svc.CreateService(context.Background(), seal.ID, service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "worker", InternalPort: 4000, Dependencies: []string{"web"}})
	if err != nil || len(worker.Dependencies) != 1 || worker.Dependencies[0] != "web" {
		t.Fatalf("create dependent service: service=%+v err=%v", worker, err)
	}

	before, err := svc.Configuration(context.Background(), seal.ID)
	if err != nil {
		t.Fatalf("configuration before generation: %v", err)
	}
	if before.Authority != "generated" || len(before.Facts) != 1 || !before.Facts[0].Detected || !before.Facts[0].Preconfigured {
		t.Fatalf("unexpected detected generated configuration: %+v", before)
	}
	generated, err := svc.SaveConfiguration(context.Background(), seal.ID, service.SaveSealConfigurationRequest{ApplyNextJSTemplate: true, ServiceID: web.ID})
	if err != nil {
		t.Fatalf("generate Next.js configuration: %v", err)
	}
	if generated.Authority != "generated" || strings.Contains(generated.DirectCompose, "services:") {
		t.Fatalf("generated configuration exposed as direct: %+v", generated)
	}
	compose, err := os.ReadFile(filepath.Join(dataDir, "seals", "1", ".tamga", "generated", "compose.yaml"))
	if err != nil || strings.Contains(string(compose), "ports:") || !strings.Contains(string(compose), "expose:") {
		t.Fatalf("generated compose must be private: %q err=%v", compose, err)
	}
	if !strings.Contains(string(compose), "web:") || !strings.Contains(string(compose), "worker:") || !strings.Contains(string(compose), "depends_on:") {
		t.Fatalf("generated compose must project every validated service and dependency: %q", compose)
	}
	dockerfile, err := os.ReadFile(filepath.Join(dataDir, "seals", "1", ".tamga", "generated", "Dockerfile"))
	if err != nil || !strings.Contains(string(dockerfile), "corepack prepare pnpm@9.15.9 --activate") || !strings.Contains(string(dockerfile), "pnpm install --frozen-lockfile") || !strings.Contains(string(dockerfile), "RUN pnpm build") || !strings.Contains(string(dockerfile), `CMD ["pnpm", "start"]`) {
		t.Fatalf("generated Dockerfile lacks pnpm frozen contract: %q err=%v", dockerfile, err)
	}
	if _, err := os.Stat(filepath.Join(checkout, "Dockerfile")); !os.IsNotExist(err) {
		t.Fatalf("generated Dockerfile leaked into customer checkout: %v", err)
	}

	direct := "services:\n  web:\n    build:\n      context: .\n"
	saved, err := svc.SaveConfiguration(context.Background(), seal.ID, service.SaveSealConfigurationRequest{ComposeYAML: direct})
	if err != nil || saved.Authority != "direct" || saved.DirectCompose != strings.TrimSpace(direct) {
		t.Fatalf("save direct configuration: configuration=%+v err=%v", saved, err)
	}
	if unchanged, err := os.ReadFile(filepath.Join(dataDir, "seals", "1", ".tamga", "generated", "compose.yaml")); err != nil || string(unchanged) != string(compose) {
		t.Fatalf("direct configuration must not replace the generated artifact: %q err=%v", unchanged, err)
	}
	if _, err := svc.SaveConfiguration(context.Background(), seal.ID, service.SaveSealConfigurationRequest{ComposeYAML: "services:\n  web:\n    ports: [\"3000:3000\"]\n"}); err == nil {
		t.Fatal("host port mapping was accepted")
	}
	reset, err := svc.SaveConfiguration(context.Background(), seal.ID, service.SaveSealConfigurationRequest{Regenerate: true})
	if err != nil || reset.Authority != "generated" || reset.DirectCompose != "" {
		t.Fatalf("reset generated authority: configuration=%+v err=%v", reset, err)
	}
	if regenerated, err := os.ReadFile(filepath.Join(dataDir, "seals", "1", ".tamga", "generated", "compose.yaml")); err != nil || !strings.Contains(string(regenerated), "worker:") || strings.Contains(string(regenerated), "ports:") {
		t.Fatalf("reset must regenerate the full private projection: %q err=%v", regenerated, err)
	}
}

func TestCreateSealServiceValidatesDeclarations(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "seals.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := service.NewSealService(db, config.Config{DataDir: t.TempDir()})
	seal, err := svc.Create(context.Background(), service.CreateSealRequest{Name: "services"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}
	repository := &domain.SealRepository{SealID: seal.ID, DisplayName: "application", RemoteURL: "https://example.invalid/application.git", Branch: "main", WorkspacePath: "repositories/application", Status: domain.ProjectSourceStatusReady}
	if err := db.CreateSealRepository(repository); err != nil {
		t.Fatalf("create repository: %v", err)
	}

	api, err := svc.CreateService(context.Background(), seal.ID, service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "api", BuildContext: "cmd/api", InternalPort: 8080})
	if err != nil {
		t.Fatalf("create api service: %v", err)
	}
	worker, err := svc.CreateService(context.Background(), seal.ID, service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "worker", BuildContext: "cmd/worker", InternalPort: 9000, Dependencies: []string{"api", "api"}})
	if err != nil {
		t.Fatalf("create dependent service: %v", err)
	}
	if worker.RepositoryID != repository.ID || worker.BuildContext != "cmd/worker" || len(worker.Dependencies) != 1 || worker.Dependencies[0] != api.Name {
		t.Fatalf("unexpected normalized worker service: %+v", worker)
	}

	cases := []struct {
		name string
		req  service.CreateSealServiceRequest
	}{
		{name: "duplicate name", req: service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "api", BuildContext: ".", InternalPort: 8081}},
		{name: "parent build context", req: service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "escape-parent", BuildContext: "../outside", InternalPort: 8081}},
		{name: "absolute build context", req: service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "escape-absolute", BuildContext: "/outside", InternalPort: 8081}},
		{name: "zero port", req: service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "zero-port", BuildContext: ".", InternalPort: 0}},
		{name: "oversized port", req: service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "large-port", BuildContext: ".", InternalPort: 65536}},
		{name: "unknown dependency", req: service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "missing-dependency", BuildContext: ".", InternalPort: 8081, Dependencies: []string{"does-not-exist"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.CreateService(context.Background(), seal.ID, tc.req); err == nil {
				t.Fatal("expected invalid service declaration to fail")
			}
		})
	}
}

func TestPinnedNextJSBlueprintRejectsIncompleteContractWithEvidence(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "seals.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	dataDir := t.TempDir()
	svc := service.NewSealService(db, config.Config{DataDir: dataDir})
	seal, err := svc.Create(context.Background(), service.CreateSealRequest{Name: "missing-contract"})
	if err != nil {
		t.Fatalf("create seal: %v", err)
	}
	repository := &domain.SealRepository{SealID: seal.ID, DisplayName: "web", RemoteURL: "https://example.invalid/web.git", Branch: "main", WorkspacePath: "repositories/web", Status: domain.ProjectSourceStatusReady}
	if err := db.CreateSealRepository(repository); err != nil {
		t.Fatalf("create repository: %v", err)
	}
	checkout := filepath.Join(dataDir, "seals", "1", "repositories", "web")
	if err := os.MkdirAll(checkout, 0755); err != nil {
		t.Fatalf("create checkout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "package.json"), []byte(`{"dependencies":{"next":"16.2.6"},"scripts":{"build":"next build"}}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
	web, err := svc.CreateService(context.Background(), seal.ID, service.CreateSealServiceRequest{RepositoryID: repository.ID, Name: "web", InternalPort: 3000})
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	configuration, err := svc.Configuration(context.Background(), seal.ID)
	if err != nil || len(configuration.Facts) != 1 || !configuration.Facts[0].Detected || configuration.Facts[0].Preconfigured {
		t.Fatalf("expected detected but unconfigured repository: configuration=%+v err=%v", configuration, err)
	}
	_, err = svc.SaveConfiguration(context.Background(), seal.ID, service.SaveSealConfigurationRequest{ApplyNextJSTemplate: true, ServiceID: web.ID})
	if err == nil {
		t.Fatal("incomplete pinned blueprint contract was accepted")
	}
	for _, evidence := range []string{"repository must be MaxLeiter/maxleiter.com", "checkout must be pinned at add180d6f8874113d02103bc5635c04059211031", "missing pnpm-lock.yaml", "build and start scripts"} {
		if !strings.Contains(err.Error(), evidence) {
			t.Fatalf("missing actionable evidence %q from error %q", evidence, err)
		}
	}
}

func writePinnedNextJSCheckout(t *testing.T, checkout string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(checkout, ".git"), 0755); err != nil {
		t.Fatalf("create git metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkout, ".git", "HEAD"), []byte("add180d6f8874113d02103bc5635c04059211031\n"), 0644); err != nil {
		t.Fatalf("write git head: %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "pnpm-lock.yaml"), []byte("lockfileVersion: '9.0'\n"), 0644); err != nil {
		t.Fatalf("write pnpm lock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkout, "package.json"), []byte(`{"dependencies":{"next":"16.2.6"},"packageManager":"pnpm@9.15.9+sha512-pinned","scripts":{"build":"concurrently \"pnpm:build-*\"","start":"next start"}}`), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}
}
