package service_test

import (
	"os"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func newTestEgressService(t *testing.T) *service.EgressService {
	t.Helper()
	dbPath := "/tmp/test_egress_service_" + t.Name() + ".db"
	os.Remove(dbPath)
	t.Cleanup(func() {
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return service.NewEgressService(db)
}

func TestEgressServiceModeGetSet(t *testing.T) {
	svc := newTestEgressService(t)

	// Migration 000014 seeds the default mode to "open".
	mode, err := svc.GetMode()
	if err != nil {
		t.Fatalf("get mode: %v", err)
	}
	if mode != domain.EgressModeOpen {
		t.Fatalf("expected default mode %q, got %q", domain.EgressModeOpen, mode)
	}

	updated, err := svc.SetMode("whitelist")
	if err != nil {
		t.Fatalf("set mode: %v", err)
	}
	if updated != domain.EgressModeWhitelist {
		t.Fatalf("expected mode %q, got %q", domain.EgressModeWhitelist, updated)
	}

	mode, err = svc.GetMode()
	if err != nil {
		t.Fatalf("get mode after set: %v", err)
	}
	if mode != domain.EgressModeWhitelist {
		t.Fatalf("get mode after set mismatch: got %q", mode)
	}

	if _, err := svc.SetMode("blacklist"); err != nil {
		t.Fatalf("set mode blacklist: %v", err)
	}
	mode, err = svc.GetMode()
	if err != nil {
		t.Fatalf("get mode after second set: %v", err)
	}
	if mode != domain.EgressModeBlacklist {
		t.Fatalf("get mode after second set mismatch: got %q", mode)
	}

	if _, err := svc.SetMode("bogus"); err == nil {
		t.Error("expected error for invalid mode")
	}
	// An invalid SetMode call must not have overwritten the last valid mode.
	mode, err = svc.GetMode()
	if err != nil {
		t.Fatalf("get mode after invalid set: %v", err)
	}
	if mode != domain.EgressModeBlacklist {
		t.Fatalf("expected mode to stay %q after rejected set, got %q", domain.EgressModeBlacklist, mode)
	}
}

func TestEgressServiceBlacklistCRUD(t *testing.T) {
	svc := newTestEgressService(t)

	// Unlike the whitelist, the blacklist has no seeded defaults.
	domains, err := svc.BlacklistDomains()
	if err != nil {
		t.Fatalf("domains: %v", err)
	}
	if len(domains) != 0 {
		t.Fatalf("expected 0 seeded blacklist domains, got %d: %v", len(domains), domains)
	}

	added, err := svc.AddBlacklist("  Evil.EXAMPLE.  ")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if added.Domain != "evil.example" {
		t.Errorf("expected normalized domain 'evil.example', got %q", added.Domain)
	}

	list, err := svc.ListBlacklist()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 domain after add, got %d", len(list))
	}

	if _, err := svc.AddBlacklist("evil.example"); err == nil {
		t.Error("expected error adding duplicate domain to blacklist")
	}

	if err := svc.RemoveBlacklist(added.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	list, err = svc.ListBlacklist()
	if err != nil {
		t.Fatalf("list after remove: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 domains after remove, got %d", len(list))
	}
}
