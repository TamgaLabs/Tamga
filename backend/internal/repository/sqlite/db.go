package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{db}, nil
}

func (db *DB) Migrate() error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (filename TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')))`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	rows, err := db.Query("SELECT filename FROM schema_migrations ORDER BY filename")
	if err != nil {
		return fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[name] = true
	}
	rows.Close()

	var tableCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT IN ('schema_migrations')").Scan(&tableCount); err != nil {
		return fmt.Errorf("count tables: %w", err)
	}

	// Legacy DB: has tables but no tracking records
	if tableCount > 0 && len(applied) == 0 {
		slog.Info("existing database without migration tracking, marking all current migrations as applied")
		for _, f := range upFiles {
			if _, err := db.Exec("INSERT OR IGNORE INTO schema_migrations (filename) VALUES (?)", f); err != nil {
				return fmt.Errorf("record migration %s: %w", f, err)
			}
		}
		return nil
	}

	for _, f := range upFiles {
		if applied[f] {
			continue
		}
		slog.Info("running migration", "file", f)
		content, err := migrationsFS.ReadFile("migrations/" + f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("exec %s: %w", f, err)
		}
		if _, err := db.Exec("INSERT INTO schema_migrations (filename) VALUES (?)", f); err != nil {
			return fmt.Errorf("record migration %s: %w", f, err)
		}
	}

	return nil
}
