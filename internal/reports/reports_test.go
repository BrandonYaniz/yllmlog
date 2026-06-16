package reports

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestGenerateReport(t *testing.T) {
	db := openTestDB(t)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	insertEvent(t, db, "auth", "warning", "failed logins", 4, now.Add(-time.Hour))
	insertEvent(t, db, "disk", "critical", "filesystem nearly full", 1, now.Add(-2*time.Hour))
	insertEvent(t, db, "old", "warning", "old event", 10, now.Add(-48*time.Hour))

	report, err := NewGenerator(db).Generate(context.Background(), KindDaily, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if report.ID == 0 {
		t.Fatal("report ID is empty")
	}
	if !strings.Contains(report.Body, "[critical] disk: filesystem nearly full") {
		t.Fatalf("report body missing critical event:\n%s", report.Body)
	}
	if !strings.Contains(report.Body, "[warning] auth: failed logins") {
		t.Fatalf("report body missing warning event:\n%s", report.Body)
	}
	if strings.Contains(report.Body, "old event") {
		t.Fatalf("report body included out-of-period event:\n%s", report.Body)
	}

	var storedBody string
	if err := db.QueryRow("SELECT body FROM reports WHERE id = ?", report.ID).Scan(&storedBody); err != nil {
		t.Fatalf("read stored report: %v", err)
	}
	if storedBody != report.Body {
		t.Fatal("stored report body differs from returned body")
	}
}

func TestGenerateEmptyReport(t *testing.T) {
	db := openTestDB(t)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)

	report, err := NewGenerator(db).Generate(context.Background(), KindWeekly, now.Add(-7*24*time.Hour), now)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !strings.Contains(report.Body, "No matching events") {
		t.Fatalf("empty report body = %q", report.Body)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
	})
	if _, err := db.Exec(`
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
CREATE TABLE reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func insertEvent(t *testing.T, db *sql.DB, service, severity, summary string, total int, lastSeen time.Time) {
	t.Helper()

	_, err := db.Exec(`
INSERT INTO events(fingerprint, service_name, severity, summary, first_seen_at, last_seen_at, total_occurrences)
VALUES(?, ?, ?, ?, ?, ?, ?);
`, service+"-"+severity+"-"+summary, service, severity, summary, lastSeen.Add(-time.Hour).UTC().Format(time.RFC3339Nano), lastSeen.UTC().Format(time.RFC3339Nano), total)
	if err != nil {
		t.Fatalf("insert event: %v", err)
	}
}
