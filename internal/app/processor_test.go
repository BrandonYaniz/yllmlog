package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestProcessorRunCycle(t *testing.T) {
	db := openTestDB(t)
	logPath := filepath.Join(t.TempDir(), "messages")
	if err := os.WriteFile(logPath, []byte("ordinary line\nignore this noise\ndisk error 123\npartial"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	if _, err := db.Exec(`
INSERT INTO watched_logs(path, service_name) VALUES(?, 'system');
INSERT INTO rules(id, name, source, matcher, pattern, action, priority, enabled)
VALUES
    (1, 'ignore noise', 'admin_created', 'contains', 'ignore this', 'ignore', 10, 1),
    (2, 'disk errors', 'system_default', 'contains', 'disk error', 'escalate', 20, 1);
`, logPath); err != nil {
		t.Fatalf("seed database: %v", err)
	}

	processor := NewProcessor(db)
	result, err := processor.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle returned error: %v", err)
	}
	if result.Lines != 3 {
		t.Fatalf("Lines = %d, want 3", result.Lines)
	}
	if result.Matched != 2 || result.Ignored != 1 || result.Events != 1 {
		t.Fatalf("result = %+v", result)
	}

	var offset int64
	if err := db.QueryRow("SELECT offset_bytes FROM log_files").Scan(&offset); err != nil {
		t.Fatalf("read offset: %v", err)
	}
	if offset != int64(len("ordinary line\nignore this noise\ndisk error 123\n")) {
		t.Fatalf("offset = %d", offset)
	}

	var occurrenceCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM event_occurrences").Scan(&occurrenceCount); err != nil {
		t.Fatalf("count occurrences: %v", err)
	}
	if occurrenceCount != 1 {
		t.Fatalf("occurrence count = %d, want 1", occurrenceCount)
	}

	var severity, summary string
	if err := db.QueryRow("SELECT severity, summary FROM events").Scan(&severity, &summary); err != nil {
		t.Fatalf("read event: %v", err)
	}
	if severity != "critical" || summary != "disk error 123" {
		t.Fatalf("event severity=%q summary=%q", severity, summary)
	}

	if err := appendFile(logPath, " completed\nanother disk error 456\n"); err != nil {
		t.Fatalf("append log: %v", err)
	}
	result, err = processor.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("second RunCycle returned error: %v", err)
	}
	if result.Lines != 2 || result.Events != 1 {
		t.Fatalf("second result = %+v", result)
	}

	if err := db.QueryRow("SELECT COUNT(*) FROM event_occurrences").Scan(&occurrenceCount); err != nil {
		t.Fatalf("count second occurrences: %v", err)
	}
	if occurrenceCount != 2 {
		t.Fatalf("occurrence count = %d, want 2", occurrenceCount)
	}
}

func TestProcessorDoesNotRepeatProcessedLines(t *testing.T) {
	db := openTestDB(t)
	logPath := filepath.Join(t.TempDir(), "messages")
	if err := os.WriteFile(logPath, []byte("panic happened\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO watched_logs(path, service_name) VALUES(?, 'kernel');
INSERT INTO rules(id, name, source, matcher, pattern, action, priority, enabled)
VALUES(1, 'panic', 'system_default', 'contains', 'panic', 'analyze', 10, 1);
`, logPath); err != nil {
		t.Fatalf("seed database: %v", err)
	}

	processor := NewProcessor(db)
	if _, err := processor.RunCycle(context.Background()); err != nil {
		t.Fatalf("first RunCycle returned error: %v", err)
	}
	result, err := processor.RunCycle(context.Background())
	if err != nil {
		t.Fatalf("second RunCycle returned error: %v", err)
	}
	if result.Lines != 0 || result.Events != 0 {
		t.Fatalf("second result = %+v", result)
	}
}

func TestProcessorPreservesOffsetsWithoutRules(t *testing.T) {
	db := openTestDB(t)
	logPath := filepath.Join(t.TempDir(), "messages")
	if err := os.WriteFile(logPath, []byte("unclassified line\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if _, err := db.Exec("INSERT INTO watched_logs(path, service_name) VALUES(?, 'system')", logPath); err != nil {
		t.Fatalf("seed watched log: %v", err)
	}

	result, err := NewProcessor(db).RunCycle(context.Background())
	if err != nil {
		t.Fatalf("RunCycle returned error: %v", err)
	}
	if result.Lines != 0 {
		t.Fatalf("Lines = %d, want 0", result.Lines)
	}

	var tracked int
	if err := db.QueryRow("SELECT COUNT(*) FROM log_files").Scan(&tracked); err != nil {
		t.Fatalf("count tracked files: %v", err)
	}
	if tracked != 0 {
		t.Fatalf("tracked files = %d, want 0", tracked)
	}
}

func appendFile(path, text string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(text)
	return err
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
CREATE TABLE watched_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    service_name TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE log_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    watched_log_id INTEGER NOT NULL REFERENCES watched_logs(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    device TEXT,
    inode TEXT,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    offset_bytes INTEGER NOT NULL DEFAULT 0,
    last_seen_at TEXT,
    UNIQUE(watched_log_id, path)
);
CREATE TABLE rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL,
    matcher TEXT NOT NULL,
    pattern TEXT NOT NULL,
    field TEXT,
    action TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 100,
    enabled INTEGER NOT NULL DEFAULT 1
);
CREATE TABLE rule_performance (
    rule_id INTEGER PRIMARY KEY REFERENCES rules(id) ON DELETE CASCADE,
    match_count INTEGER NOT NULL DEFAULT 0,
    last_matched_at TEXT,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    fingerprint TEXT NOT NULL UNIQUE,
    service_name TEXT,
    severity TEXT NOT NULL DEFAULT 'unknown',
    summary TEXT,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    total_occurrences INTEGER NOT NULL DEFAULT 0,
    today_occurrences INTEGER NOT NULL DEFAULT 0,
    last_hour_occurrences INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE event_occurrences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    log_file_id INTEGER REFERENCES log_files(id) ON DELETE SET NULL,
    occurred_at TEXT NOT NULL,
    line TEXT NOT NULL
);`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}
