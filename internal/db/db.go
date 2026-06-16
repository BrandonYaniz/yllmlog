package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const migrationLedger = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

// Open creates parent directories as needed and opens a SQLite database.
func Open(path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	database, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}
	return database, nil
}

// ApplyMigrations applies all pending .sql files in dir from migrationsFS.
func ApplyMigrations(ctx context.Context, database *sql.DB, migrationsFS fs.FS, dir string) error {
	if database == nil {
		return errors.New("database is required")
	}
	if _, err := database.ExecContext(ctx, migrationLedger); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	migrations, err := loadMigrations(migrationsFS, dir)
	if err != nil {
		return err
	}
	for _, migration := range migrations {
		applied, err := migrationApplied(ctx, database, migration.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(ctx, database, migration); err != nil {
			return err
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func loadMigrations(migrationsFS fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	seen := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version, err := migrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}
		if previous, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d in %q and %q", version, previous, entry.Name())
		}
		seen[version] = entry.Name()

		content, err := fs.ReadFile(migrationsFS, filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		migrations = append(migrations, migration{
			version: version,
			name:    entry.Name(),
			sql:     string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})
	return migrations, nil
}

func migrationVersion(name string) (int, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("migration %q must start with a numeric prefix", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("migration %q must start with a positive numeric prefix", name)
	}
	return version, nil
}

func migrationApplied(ctx context.Context, database *sql.DB, version int) (bool, error) {
	var count int
	err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check migration %d: %w", version, err)
	}
	return count > 0, nil
}

func applyMigration(ctx context.Context, database *sql.DB, migration migration) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", migration.name, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, migration.sql); err != nil {
		return fmt.Errorf("apply migration %s: %w", migration.name, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, name) VALUES(?, ?)", migration.version, migration.name); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.name, err)
	}
	return nil
}
