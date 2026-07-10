package service_test

import (
	"os"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestIdleTimeoutServiceGetSet(t *testing.T) {
	dbPath := "/tmp/test_idle_timeout_service.db"
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

	svc := service.NewIdleTimeoutService(db)

	// Migration seeds Never (0) by default, on both fresh and existing
	// installs.
	it, err := svc.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if it.TimeoutSeconds != 0 {
		t.Fatalf("expected seeded default of 0 (Never), got %d", it.TimeoutSeconds)
	}

	updated, err := svc.Set(1800)
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if updated.TimeoutSeconds != 1800 {
		t.Fatalf("unexpected updated value: %+v", updated)
	}

	it, err = svc.Get()
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if it.TimeoutSeconds != 1800 {
		t.Fatalf("get after set mismatch: %+v", it)
	}

	// Setting back to 0 (Never) is a valid, supported value, not an error.
	back, err := svc.Set(0)
	if err != nil {
		t.Fatalf("set back to never: %v", err)
	}
	if back.TimeoutSeconds != 0 {
		t.Fatalf("expected 0 after resetting to never, got %d", back.TimeoutSeconds)
	}

	if _, err := svc.Set(-1); err == nil {
		t.Error("expected error for negative timeout_seconds")
	}
}
