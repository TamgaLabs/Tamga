package service_test

import (
	"os"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestWhitelistServiceCRUD(t *testing.T) {
	dbPath := "/tmp/test_whitelist_service.db"
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	svc := service.NewWhitelistService(db)

	// Migration seeds the defaults.
	domains, err := svc.Domains()
	if err != nil {
		t.Fatalf("domains: %v", err)
	}
	if len(domains) != 3 {
		t.Fatalf("expected 3 seeded domains, got %d: %v", len(domains), domains)
	}

	added, err := svc.Add("  Example.COM.  ")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if added.Domain != "example.com" {
		t.Errorf("expected normalized domain 'example.com', got %q", added.Domain)
	}

	list, err := svc.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 4 {
		t.Fatalf("expected 4 domains after add, got %d", len(list))
	}

	if err := svc.Remove(added.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	list, err = svc.List()
	if err != nil {
		t.Fatalf("list after remove: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 domains after remove, got %d", len(list))
	}
}
