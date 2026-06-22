package events

import (
	"context"
	"testing"
	"time"
)

func TestListAndGetEvents(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)

	critical, err := store.Record(context.Background(), Occurrence{
		Service: "disk", Severity: "critical", Summary: "disk full", Line: "disk full 95", At: now,
	})
	if err != nil {
		t.Fatalf("record critical event: %v", err)
	}
	if _, err := store.Record(context.Background(), Occurrence{
		Service: "auth", Severity: "warning", Summary: "login failed", Line: "login failed 1", At: now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("record warning event: %v", err)
	}

	listed, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(listed) != 2 || listed[0].ID != critical.ID {
		t.Fatalf("listed events = %+v", listed)
	}

	detail, err := store.Get(context.Background(), critical.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if detail.Event.Summary != "disk full" || len(detail.Occurrences) != 1 {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.Occurrences[0].Line != "disk full 95" {
		t.Fatalf("occurrence = %+v", detail.Occurrences[0])
	}
}
