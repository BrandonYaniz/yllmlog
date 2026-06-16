package db

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"
)

func TestApplyMigrations(t *testing.T) {
	database := openTestDB(t)
	migrations := fstest.MapFS{
		"migrations/002_second.sql": {Data: []byte(`CREATE TABLE second_table (id INTEGER PRIMARY KEY);`)},
		"migrations/001_first.sql":  {Data: []byte(`CREATE TABLE first_table (id INTEGER PRIMARY KEY);`)},
	}

	if err := ApplyMigrations(context.Background(), database, migrations, "migrations"); err != nil {
		t.Fatalf("ApplyMigrations returned error: %v", err)
	}
	if err := ApplyMigrations(context.Background(), database, migrations, "migrations"); err != nil {
		t.Fatalf("second ApplyMigrations returned error: %v", err)
	}

	assertTableExists(t, database, "first_table")
	assertTableExists(t, database, "second_table")

	var applied int
	if err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&applied); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if applied != 2 {
		t.Fatalf("applied migrations = %d, want 2", applied)
	}
}

func TestApplyMigrationsRejectsDuplicateVersions(t *testing.T) {
	database := openTestDB(t)
	migrations := fstest.MapFS{
		"migrations/001_first.sql": {Data: []byte(`CREATE TABLE first_table (id INTEGER PRIMARY KEY);`)},
		"migrations/001_other.sql": {Data: []byte(`CREATE TABLE other_table (id INTEGER PRIMARY KEY);`)},
	}

	if err := ApplyMigrations(context.Background(), database, migrations, "migrations"); err == nil {
		t.Fatal("ApplyMigrations accepted duplicate versions")
	}
}

func TestOpenCreatesDatabase(t *testing.T) {
	database, err := Open(t.TempDir() + "/state/yllmlog.db")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer database.Close()

	if err := database.Ping(); err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	return database
}

func assertTableExists(t *testing.T, database *sql.DB, table string) {
	t.Helper()

	var name string
	err := database.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&name)
	if err != nil {
		t.Fatalf("table %q does not exist: %v", table, err)
	}
}
