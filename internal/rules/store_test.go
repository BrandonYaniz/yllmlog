package rules

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestLoadEnabled(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
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
INSERT INTO rules(name, source, matcher, pattern, field, action, priority, enabled)
VALUES
    ('enabled', 'admin_created', 'field_equals', 'auth', 'facility', 'report', 10, 1),
    ('disabled', 'system_default', 'contains', 'noise', NULL, 'ignore', 20, 0);
`); err != nil {
		t.Fatalf("create rules: %v", err)
	}

	loaded, err := LoadEnabled(context.Background(), db)
	if err != nil {
		t.Fatalf("LoadEnabled returned error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("len(loaded) = %d, want 1", len(loaded))
	}
	if loaded[0].Matcher != MatcherFieldEquals || loaded[0].Field != "facility" {
		t.Fatalf("loaded rule = %+v", loaded[0])
	}
}
