package logs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRefreshFilesTracksOffsetAndTruncation(t *testing.T) {
	store := NewStore(openTestDB(t))
	path := writeLog(t, "messages", "first\nsecond\n")
	watched, err := store.Add(context.Background(), path, "system")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	states, err := store.RefreshFiles(context.Background(), watched)
	if err != nil {
		t.Fatalf("RefreshFiles returned error: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("len(states) = %d, want 1", len(states))
	}
	if states[0].Rotation != RotationNew {
		t.Fatalf("initial Rotation = %q", states[0].Rotation)
	}

	if err := store.UpdateOffset(context.Background(), states[0].ID, states[0].SizeBytes); err != nil {
		t.Fatalf("UpdateOffset returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatalf("truncate log: %v", err)
	}

	states, err = store.RefreshFiles(context.Background(), watched)
	if err != nil {
		t.Fatalf("RefreshFiles after truncate returned error: %v", err)
	}
	if states[0].Rotation != RotationTruncated {
		t.Fatalf("Rotation = %q, want %q", states[0].Rotation, RotationTruncated)
	}
	if states[0].OffsetBytes != 0 {
		t.Fatalf("OffsetBytes = %d, want 0", states[0].OffsetBytes)
	}
}

func TestRefreshFilesExpandsGlobAndReportsMissing(t *testing.T) {
	store := NewStore(openTestDB(t))
	dir := t.TempDir()
	first := filepath.Join(dir, "first.log")
	second := filepath.Join(dir, "second.log")
	if err := os.WriteFile(first, []byte("first\n"), 0o600); err != nil {
		t.Fatalf("write first log: %v", err)
	}
	if err := os.WriteFile(second, []byte("second\n"), 0o600); err != nil {
		t.Fatalf("write second log: %v", err)
	}

	watched, err := store.Add(context.Background(), filepath.Join(dir, "*.log"), "")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	states, err := store.RefreshFiles(context.Background(), watched)
	if err != nil {
		t.Fatalf("RefreshFiles returned error: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("len(states) = %d, want 2", len(states))
	}

	if err := os.Remove(second); err != nil {
		t.Fatalf("remove second log: %v", err)
	}
	states, err = store.RefreshFiles(context.Background(), watched)
	if err != nil {
		t.Fatalf("RefreshFiles after remove returned error: %v", err)
	}

	foundMissing := false
	for _, state := range states {
		if state.Path == second && state.Rotation == RotationMissing {
			foundMissing = true
		}
	}
	if !foundMissing {
		t.Fatalf("missing state for %s not found: %+v", second, states)
	}
}

func TestRefreshFilesDetectsReplacement(t *testing.T) {
	store := NewStore(openTestDB(t))
	path := writeLog(t, "messages", "first\n")
	watched, err := store.Add(context.Background(), path, "")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	states, err := store.RefreshFiles(context.Background(), watched)
	if err != nil {
		t.Fatalf("RefreshFiles returned error: %v", err)
	}
	initial := states[0]

	replacement := path + ".new"
	if err := os.WriteFile(replacement, []byte("replacement\n"), 0o600); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("rename replacement: %v", err)
	}

	states, err = store.RefreshFiles(context.Background(), watched)
	if err != nil {
		t.Fatalf("RefreshFiles after replacement returned error: %v", err)
	}
	if states[0].ID == 0 {
		t.Fatal("state ID is empty")
	}
	if initial.Device == states[0].Device && initial.Inode == states[0].Inode {
		t.Skip("filesystem reused the same file identity for replacement")
	}
	if states[0].Rotation != RotationReplaced {
		t.Fatalf("Rotation = %q, want %q", states[0].Rotation, RotationReplaced)
	}
}

func writeLog(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return path
}
