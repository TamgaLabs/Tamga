package service

import (
	"os"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

func TestResourceLimitServiceGetSet(t *testing.T) {
	dbPath := "/tmp/test_resource_limit_service.db"
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

	svc := NewResourceLimitService(db)

	// Migration seeds a non-zero default.
	rl, err := svc.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rl.MemoryBytes <= 0 || rl.NanoCPUs <= 0 {
		t.Fatalf("expected non-zero seeded default, got %+v", rl)
	}

	updated, err := svc.Set(2<<30, 2_000_000_000)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if updated.MemoryBytes != 2<<30 || updated.NanoCPUs != 2_000_000_000 {
		t.Fatalf("unexpected updated values: %+v", updated)
	}

	rl, err = svc.Get()
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if rl.MemoryBytes != 2<<30 || rl.NanoCPUs != 2_000_000_000 {
		t.Fatalf("get after set mismatch: %+v", rl)
	}

	if _, err := svc.Set(0, 1_000_000_000); err == nil {
		t.Error("expected error for zero memory_bytes")
	}
	if _, err := svc.Set(1<<30, 0); err == nil {
		t.Error("expected error for zero nano_cpus")
	}
	if _, err := svc.Set(-1, 1_000_000_000); err == nil {
		t.Error("expected error for negative memory_bytes")
	}
}
