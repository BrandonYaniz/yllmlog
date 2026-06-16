package reports

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Kind string

const (
	KindDaily  Kind = "daily"
	KindWeekly Kind = "weekly"
)

type Report struct {
	ID          int64  `json:"id"`
	Kind        Kind   `json:"kind"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Body        string `json:"body"`
}

type Generator struct {
	db *sql.DB
}

func NewGenerator(db *sql.DB) Generator {
	return Generator{db: db}
}

func (g Generator) Generate(ctx context.Context, kind Kind, start, end time.Time) (Report, error) {
	if kind != KindDaily && kind != KindWeekly {
		return Report{}, fmt.Errorf("unsupported report kind %q", kind)
	}
	if !start.Before(end) {
		return Report{}, errors.New("report start must be before end")
	}

	events, err := g.loadEvents(ctx, start, end)
	if err != nil {
		return Report{}, err
	}
	body := render(kind, start, end, events)
	startText := start.UTC().Format(time.RFC3339)
	endText := end.UTC().Format(time.RFC3339)

	result, err := g.db.ExecContext(ctx, `
INSERT INTO reports(kind, period_start, period_end, body)
VALUES(?, ?, ?, ?);
`, kind, startText, endText, body)
	if err != nil {
		return Report{}, fmt.Errorf("insert report: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Report{}, fmt.Errorf("read report id: %w", err)
	}

	return Report{
		ID:          id,
		Kind:        kind,
		PeriodStart: startText,
		PeriodEnd:   endText,
		Body:        body,
	}, nil
}

type eventSummary struct {
	ServiceName      string
	Severity         string
	Summary          string
	TotalOccurrences int64
	LastSeenAt       string
}

func (g Generator) loadEvents(ctx context.Context, start, end time.Time) ([]eventSummary, error) {
	rows, err := g.db.QueryContext(ctx, `
SELECT COALESCE(service_name, ''), severity, COALESCE(summary, ''), total_occurrences, last_seen_at
FROM events
WHERE last_seen_at >= ? AND last_seen_at < ?
ORDER BY
    CASE severity
        WHEN 'critical' THEN 0
        WHEN 'error' THEN 1
        WHEN 'warning' THEN 2
        ELSE 3
    END,
    total_occurrences DESC,
    last_seen_at DESC;
`, start.UTC().Format(time.RFC3339Nano), end.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, fmt.Errorf("load report events: %w", err)
	}
	defer rows.Close()

	var events []eventSummary
	for rows.Next() {
		var event eventSummary
		if err := rows.Scan(&event.ServiceName, &event.Severity, &event.Summary, &event.TotalOccurrences, &event.LastSeenAt); err != nil {
			return nil, fmt.Errorf("scan report event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load report events: %w", err)
	}
	return events, nil
}

func render(kind Kind, start, end time.Time, events []eventSummary) string {
	var body bytes.Buffer
	fmt.Fprintf(&body, "yllmlog %s report\n", kind)
	fmt.Fprintf(&body, "Period: %s to %s\n\n", start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339))
	if len(events) == 0 {
		body.WriteString("No matching events were recorded during this period.\n")
		return body.String()
	}

	for _, event := range events {
		service := event.ServiceName
		if service == "" {
			service = "unknown-service"
		}
		summary := strings.TrimSpace(event.Summary)
		if summary == "" {
			summary = "No summary yet"
		}
		fmt.Fprintf(&body, "- [%s] %s: %s (%d occurrences, last seen %s)\n", event.Severity, service, summary, event.TotalOccurrences, event.LastSeenAt)
	}
	return body.String()
}
