package events

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type OccurrenceDetail struct {
	ID         int64  `json:"id"`
	LogFileID  int64  `json:"log_file_id,omitempty"`
	OccurredAt string `json:"occurred_at"`
	Line       string `json:"line"`
}

type Detail struct {
	Event       Event              `json:"event"`
	Occurrences []OccurrenceDetail `json:"occurrences"`
}

func (s Store) List(ctx context.Context) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, fingerprint, COALESCE(service_name, ''), severity, COALESCE(summary, ''), first_seen_at, last_seen_at,
       total_occurrences, today_occurrences, last_hour_occurrences
FROM events
ORDER BY
    CASE severity
        WHEN 'critical' THEN 0
        WHEN 'error' THEN 1
        WHEN 'warning' THEN 2
        ELSE 3
    END,
    last_seen_at DESC;
`)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var listed []Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		listed = append(listed, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return listed, nil
}

func (s Store) Get(ctx context.Context, id int64) (Detail, error) {
	if id <= 0 {
		return Detail{}, errors.New("event id is required")
	}

	event, err := scanEvent(s.db.QueryRowContext(ctx, `
SELECT id, fingerprint, COALESCE(service_name, ''), severity, COALESCE(summary, ''), first_seen_at, last_seen_at,
       total_occurrences, today_occurrences, last_hour_occurrences
FROM events
WHERE id = ?;
`, id))
	if err != nil {
		return Detail{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, COALESCE(log_file_id, 0), occurred_at, line
FROM event_occurrences
WHERE event_id = ?
ORDER BY occurred_at DESC, id DESC
LIMIT 100;
`, id)
	if err != nil {
		return Detail{}, fmt.Errorf("list event occurrences: %w", err)
	}
	defer rows.Close()

	var occurrences []OccurrenceDetail
	for rows.Next() {
		var occurrence OccurrenceDetail
		if err := rows.Scan(&occurrence.ID, &occurrence.LogFileID, &occurrence.OccurredAt, &occurrence.Line); err != nil {
			return Detail{}, fmt.Errorf("scan event occurrence: %w", err)
		}
		occurrences = append(occurrences, occurrence)
	}
	if err := rows.Err(); err != nil {
		return Detail{}, fmt.Errorf("list event occurrences: %w", err)
	}
	return Detail{Event: event, Occurrences: occurrences}, nil
}

func scanEvent(row rowScanner) (Event, error) {
	var event Event
	if err := row.Scan(&event.ID, &event.Fingerprint, &event.ServiceName, &event.Severity, &event.Summary, &event.FirstSeenAt, &event.LastSeenAt, &event.TotalOccurrences, &event.TodayOccurrences, &event.LastHourOccurrences); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, err
		}
		return Event{}, fmt.Errorf("scan event: %w", err)
	}
	return event, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}
