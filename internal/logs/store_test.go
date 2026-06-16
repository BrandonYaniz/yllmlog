package logs

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestStoreAddListRemove(t *testing.T) {
	store := NewStore(openTestDB(t))

	watched, err := store.Add(context.Background(), "/var/log/messages", "system")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if watched.Path != "/var/log/messages" {
		t.Fatalf("Path = %q", watched.Path)
	}
	if watched.ServiceName != "system" {
		t.Fatalf("ServiceName = %q", watched.ServiceName)
	}
	if !watched.Enabled {
		t.Fatal("watched log is not enabled")
	}

	watchedAgain, err := store.Add(context.Background(), "/var/log/messages", "syslog")
	if err != nil {
		t.Fatalf("second Add returned error: %v", err)
	}
	if watchedAgain.ID != watched.ID {
		t.Fatalf("updated ID = %d, want %d", watchedAgain.ID, watched.ID)
	}
	if watchedAgain.ServiceName != "syslog" {
		t.Fatalf("updated ServiceName = %q", watchedAgain.ServiceName)
	}

	watchedLogs, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(watchedLogs) != 1 {
		t.Fatalf("len(List) = %d, want 1", len(watchedLogs))
	}

	if err := store.Remove(context.Background(), "/var/log/messages"); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	watchedLogs, err = store.List(context.Background())
	if err != nil {
		t.Fatalf("List after remove returned error: %v", err)
	}
	if len(watchedLogs) != 0 {
		t.Fatalf("len(List) after remove = %d, want 0", len(watchedLogs))
	}
}

func TestStoreRejectsRelativePath(t *testing.T) {
	store := NewStore(openTestDB(t))
	if _, err := store.Add(context.Background(), "messages", ""); err == nil {
		t.Fatal("Add accepted relative path")
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	if _, err := db.Exec(`
CREATE TABLE watched_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    service_name TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`); err != nil {
		t.Fatalf("create watched_logs: %v", err)
	}
	return db
}
