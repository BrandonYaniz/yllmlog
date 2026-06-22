package events

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Event struct {
	ID                  int64  `json:"id"`
	Fingerprint         string `json:"fingerprint"`
	ServiceName         string `json:"service_name,omitempty"`
	Severity            string `json:"severity"`
	Summary             string `json:"summary,omitempty"`
	FirstSeenAt         string `json:"first_seen_at"`
	LastSeenAt          string `json:"last_seen_at"`
	TotalOccurrences    int64  `json:"total_occurrences"`
	TodayOccurrences    int64  `json:"today_occurrences"`
	LastHourOccurrences int64  `json:"last_hour_occurrences"`
}

type Occurrence struct {
	LogFileID int64
	Service   string
	Severity  string
	Summary   string
	Line      string
	At        time.Time
}

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func NewStore(db *sql.DB) Store {
	return Store{db: db, now: time.Now}
}

func NewStoreWithClock(db *sql.DB, now func() time.Time) Store {
	return Store{db: db, now: now}
}

func (s Store) Record(ctx context.Context, occurrence Occurrence) (Event, error) {
	if occurrence.At.IsZero() {
		occurrence.At = s.now()
	}
	counterTime := s.now()
	if occurrence.Severity == "" {
		occurrence.Severity = "unknown"
	}

	fingerprint := Fingerprint(occurrence.Service, occurrence.Line)
	occurredAt := occurrence.At.UTC().Format(time.RFC3339Nano)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, fmt.Errorf("begin event record: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO events(fingerprint, service_name, severity, summary, first_seen_at, last_seen_at, total_occurrences, today_occurrences, last_hour_occurrences)
VALUES(?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, 0, 0, 0)
ON CONFLICT(fingerprint) DO NOTHING;
`, fingerprint, occurrence.Service, occurrence.Severity, occurrence.Summary, occurredAt, occurredAt); err != nil {
		return Event{}, fmt.Errorf("upsert event: %w", err)
	}

	event, err := getEventByFingerprint(ctx, tx, fingerprint)
	if err != nil {
		return Event{}, err
	}

	var logFileID any
	if occurrence.LogFileID > 0 {
		logFileID = occurrence.LogFileID
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO event_occurrences(event_id, log_file_id, occurred_at, line)
VALUES(?, ?, ?, ?);
`, event.ID, logFileID, occurredAt, occurrence.Line); err != nil {
		return Event{}, fmt.Errorf("insert event occurrence: %w", err)
	}

	if err := updateCounters(ctx, tx, event.ID, counterTime); err != nil {
		return Event{}, err
	}
	event, err = getEventByFingerprint(ctx, tx, fingerprint)
	if err != nil {
		return Event{}, err
	}

	if err := tx.Commit(); err != nil {
		return Event{}, fmt.Errorf("commit event record: %w", err)
	}
	return event, nil
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getEventByFingerprint(ctx context.Context, db queryer, fingerprint string) (Event, error) {
	row := db.QueryRowContext(ctx, `
SELECT id, fingerprint, COALESCE(service_name, ''), severity, COALESCE(summary, ''), first_seen_at, last_seen_at,
       total_occurrences, today_occurrences, last_hour_occurrences
FROM events
WHERE fingerprint = ?;
`, fingerprint)
	return scanEvent(row)
}

func updateCounters(ctx context.Context, tx *sql.Tx, eventID int64, now time.Time) error {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC().Format(time.RFC3339Nano)
	lastHourStart := now.Add(-time.Hour).UTC().Format(time.RFC3339Nano)

	_, err := tx.ExecContext(ctx, `
UPDATE events
SET first_seen_at = (SELECT MIN(occurred_at) FROM event_occurrences WHERE event_id = ?),
    last_seen_at = (SELECT MAX(occurred_at) FROM event_occurrences WHERE event_id = ?),
    total_occurrences = (SELECT COUNT(*) FROM event_occurrences WHERE event_id = ?),
    today_occurrences = (SELECT COUNT(*) FROM event_occurrences WHERE event_id = ? AND occurred_at >= ?),
    last_hour_occurrences = (SELECT COUNT(*) FROM event_occurrences WHERE event_id = ? AND occurred_at >= ?)
WHERE id = ?;
`, eventID, eventID, eventID, eventID, todayStart, eventID, lastHourStart, eventID)
	if err != nil {
		return fmt.Errorf("update event counters: %w", err)
	}
	return nil
}
