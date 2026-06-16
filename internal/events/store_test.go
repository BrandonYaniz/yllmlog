package events

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestNormalizeAndFingerprint(t *testing.T) {
	left := "Jan  2 03:04:05 host app[123]: failed login from 10.0.0.12"
	right := "Jan  3 04:05:06 host app[456]: failed login from 10.0.0.99"

	if Normalize(left) != Normalize(right) {
		t.Fatalf("Normalize mismatch:\n%s\n%s", Normalize(left), Normalize(right))
	}
	if Fingerprint("auth", left) != Fingerprint("auth", right) {
		t.Fatal("fingerprints for equivalent lines differ")
	}
	if Fingerprint("auth", left) == Fingerprint("mail", left) {
		t.Fatal("fingerprints ignored service name")
	}
}

func TestRecordUpdatesOccurrenceCounters(t *testing.T) {
	db := openTestDB(t)
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	store := NewStoreWithClock(db, func() time.Time { return now })

	event, err := store.Record(context.Background(), Occurrence{
		Service:  "auth",
		Severity: "warning",
		Summary:  "failed login",
		Line:     "Jun 16 11:10:00 host sshd[111]: failed login from 10.0.0.1",
		At:       now.Add(-50 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Record returned error: %v", err)
	}
	if event.TotalOccurrences != 1 || event.TodayOccurrences != 1 || event.LastHourOccurrences != 1 {
		t.Fatalf("counters after first record = %+v", event)
	}

	event, err = store.Record(context.Background(), Occurrence{
		Service:  "auth",
		Severity: "warning",
		Line:     "Jun 16 11:30:00 host sshd[222]: failed login from 10.0.0.2",
		At:       now.Add(-30 * time.Minute),
	})
	if err != nil {
		t.Fatalf("second Record returned error: %v", err)
	}
	if event.TotalOccurrences != 2 {
		t.Fatalf("TotalOccurrences = %d, want 2", event.TotalOccurrences)
	}
	if event.TodayOccurrences != 2 {
		t.Fatalf("TodayOccurrences = %d, want 2", event.TodayOccurrences)
	}
	if event.LastHourOccurrences != 2 {
		t.Fatalf("LastHourOccurrences = %d, want 2", event.LastHourOccurrences)
	}

	event, err = store.Record(context.Background(), Occurrence{
		Service:  "auth",
		Severity: "warning",
		Line:     "Jun 16 09:00:00 host sshd[333]: failed login from 10.0.0.3",
		At:       now.Add(-3 * time.Hour),
	})
	if err != nil {
		t.Fatalf("third Record returned error: %v", err)
	}
	if event.TotalOccurrences != 3 {
		t.Fatalf("TotalOccurrences = %d, want 3", event.TotalOccurrences)
	}
	if event.TodayOccurrences != 3 {
		t.Fatalf("TodayOccurrences = %d, want 3", event.TodayOccurrences)
	}
	if event.LastHourOccurrences != 2 {
		t.Fatalf("LastHourOccurrences = %d, want 2", event.LastHourOccurrences)
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
CREATE TABLE event_occurrences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    log_file_id INTEGER,
    occurred_at TEXT NOT NULL,
    line TEXT NOT NULL
);`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}
