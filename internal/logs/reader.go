package logs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Line is a complete line read from a tracked file and its ending byte offset.
type Line struct {
	Text      string
	EndOffset int64
}

// ReadCompleteLines reads newline-terminated lines starting at the stored offset.
// A trailing partial line is left unread until a later cycle completes it.
func ReadCompleteLines(state FileState) ([]Line, error) {
	if state.Rotation == RotationMissing {
		return nil, nil
	}
	if state.OffsetBytes < 0 {
		return nil, errors.New("log offset must not be negative")
	}

	file, err := os.Open(state.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	if _, err := file.Seek(state.OffsetBytes, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek log file: %w", err)
	}

	reader := bufio.NewReader(file)
	offset := state.OffsetBytes
	var lines []Line
	for {
		raw, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			// ReadString may return data with EOF; without a newline it is partial.
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read log file: %w", err)
		}

		offset += int64(len(raw))
		text := strings.TrimSuffix(raw, "\n")
		text = strings.TrimSuffix(text, "\r")
		lines = append(lines, Line{Text: text, EndOffset: offset})
	}
	return lines, nil
}
