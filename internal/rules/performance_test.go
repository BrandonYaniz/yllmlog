package rules

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRecordMatch(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	if _, err := db.Exec(`
CREATE TABLE rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL,
    pattern TEXT NOT NULL,
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
INSERT INTO rules(id, name, source, pattern, action) VALUES(1, 'test', 'system_default', 'panic', 'analyze');
`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	if err := RecordMatch(context.Background(), db, 1); err != nil {
		t.Fatalf("RecordMatch returned error: %v", err)
	}
	if err := RecordMatch(context.Background(), db, 1); err != nil {
		t.Fatalf("second RecordMatch returned error: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT match_count FROM rule_performance WHERE rule_id = 1").Scan(&count); err != nil {
		t.Fatalf("read match count: %v", err)
	}
	if count != 2 {
		t.Fatalf("match_count = %d, want 2", count)
	}
}
