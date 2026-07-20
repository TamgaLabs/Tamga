package sqlite_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

func openTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	// Metric repository tests exercise project-owned rows with several stable
	// fixture IDs. Seed real FK parents rather than relying on the retired
	// unconstrained metric ownership model.
	if _, err := db.Exec(`INSERT INTO seals (id, name) VALUES (1, 'metrics-fixture');
		INSERT INTO projects (id, seal_id, name) VALUES
			(1, 1, 'one'), (2, 1, 'two'), (7, 1, 'seven'),
			(40, 1, 'forty'), (42, 1, 'forty-two');`); err != nil {
		t.Fatalf("seed project ownership fixtures: %v", err)
	}
	return db
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
