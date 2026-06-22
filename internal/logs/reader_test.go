package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCompleteLinesPreservesPartialLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages")
	if err := os.WriteFile(path, []byte("first\npartial"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	lines, err := ReadCompleteLines(FileState{Path: path})
	if err != nil {
		t.Fatalf("ReadCompleteLines returned error: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want 1", len(lines))
	}
	if lines[0].Text != "first" || lines[0].EndOffset != 6 {
		t.Fatalf("first line = %+v", lines[0])
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open log for append: %v", err)
	}
	if _, err := file.WriteString(" line\n"); err != nil {
		file.Close()
		t.Fatalf("append log: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close log: %v", err)
	}

	lines, err = ReadCompleteLines(FileState{Path: path, OffsetBytes: 6})
	if err != nil {
		t.Fatalf("second ReadCompleteLines returned error: %v", err)
	}
	if len(lines) != 1 || lines[0].Text != "partial line" {
		t.Fatalf("completed line = %+v", lines)
	}
}

func TestReadCompleteLinesStartsAtOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages")
	if err := os.WriteFile(path, []byte("old\nnew\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	lines, err := ReadCompleteLines(FileState{Path: path, OffsetBytes: 4})
	if err != nil {
		t.Fatalf("ReadCompleteLines returned error: %v", err)
	}
	if len(lines) != 1 || lines[0].Text != "new" || lines[0].EndOffset != 8 {
		t.Fatalf("lines = %+v", lines)
	}
}
