package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestSealServiceCreateEstablishesEmptyOwnedWorkspace(t *testing.T) {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "seals.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	dataDir := t.TempDir()
	gitInvocation := filepath.Join(t.TempDir(), "git-invoked")
	gitBin := t.TempDir()
	if err := os.WriteFile(filepath.Join(gitBin, "git"), []byte("#!/bin/sh\ntouch \"$TAMGA_TEST_GIT_INVOCATION\"\nexit 99\n"), 0755); err != nil {
		t.Fatalf("write git sentinel: %v", err)
	}
	t.Setenv("TAMGA_TEST_GIT_INVOCATION", gitInvocation)
	t.Setenv("PATH", gitBin+string(filepath.ListSeparator)+os.Getenv("PATH"))
	svc := service.NewSealService(db, config.Config{DataDir: dataDir})
	seal, err := svc.Create(context.Background(), service.CreateSealRequest{Name: "empty seal", Domain: "empty.example.test"})
	if err != nil {
		t.Fatalf("create empty seal: %v", err)
	}
	if seal.ID == 0 {
		t.Fatal("expected persisted seal ID")
	}
	if seal.Status != domain.ProjectStatusConfiguring || seal.SourceType != domain.SourceTypeEmpty {
		t.Fatalf("unexpected empty Seal lifecycle state: status=%q source_type=%q", seal.Status, seal.SourceType)
	}

	persisted, err := db.FindSeal(seal.ID)
	if err != nil {
		t.Fatalf("find persisted seal: %v", err)
	}
	if persisted.Name != "empty seal" || persisted.Domain != "empty.example.test" {
		t.Fatalf("unexpected persisted seal: %+v", persisted)
	}
	if persisted.Status != domain.ProjectStatusConfiguring || persisted.SourceType != domain.SourceTypeEmpty {
		t.Fatalf("unexpected persisted Seal lifecycle state: status=%q source_type=%q", persisted.Status, persisted.SourceType)
	}

	workspace := filepath.Join(dataDir, "seals", "1")
	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		t.Fatalf("expected Seal workspace directory: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(gitInvocation); !os.IsNotExist(err) {
		t.Fatalf("Seal creation must not invoke git: stat error=%v", err)
	}
	compose, err := os.ReadFile(filepath.Join(workspace, ".tamga", "generated", "compose.yaml"))
	if err != nil {
		t.Fatalf("read Tamga-owned empty Seal compose file: %v", err)
	}
	if string(compose) != "services: {}\n" {
		t.Fatalf("unexpected empty Seal compose file: %q", compose)
	}
	var parsed struct {
		Services map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal(compose, &parsed); err != nil {
		t.Fatalf("parse empty Seal compose file: %v", err)
	}
	if parsed.Services == nil || len(parsed.Services) != 0 {
		t.Fatalf("expected parsed services: {}, got %#v", parsed.Services)
	}
	if _, err := os.Stat(filepath.Join(workspace, "compose.yaml")); !os.IsNotExist(err) {
		t.Fatalf("generated configuration must stay out of the default workspace: %v", err)
	}

	for _, prohibited := range []string{".git", "sources"} {
		if _, err := os.Stat(filepath.Join(workspace, prohibited)); !os.IsNotExist(err) {
			t.Errorf("empty Seal creation must not create %s; stat error=%v", prohibited, err)
		}
	}
}
