package logs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

type RotationState string

const (
	RotationNone      RotationState = "none"
	RotationNew       RotationState = "new"
	RotationMissing   RotationState = "missing"
	RotationReplaced  RotationState = "replaced"
	RotationTruncated RotationState = "truncated"
)

type FileState struct {
	ID           int64         `json:"id"`
	WatchedLogID int64         `json:"watched_log_id"`
	Path         string        `json:"path"`
	Device       string        `json:"device"`
	Inode        string        `json:"inode"`
	SizeBytes    int64         `json:"size_bytes"`
	OffsetBytes  int64         `json:"offset_bytes"`
	Rotation     RotationState `json:"rotation"`
}

func (s Store) RefreshFiles(ctx context.Context, watched WatchedLog) ([]FileState, error) {
	paths, err := expandPath(watched.Path)
	if err != nil {
		return nil, err
	}

	states := make([]FileState, 0, len(paths))
	for _, path := range paths {
		state, err := s.refreshFile(ctx, watched.ID, path)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}

	missing, err := s.missingFiles(ctx, watched.ID, paths)
	if err != nil {
		return nil, err
	}
	states = append(states, missing...)
	return states, nil
}

func (s Store) UpdateOffset(ctx context.Context, fileID int64, offset int64) error {
	if fileID <= 0 {
		return errors.New("file id is required")
	}
	if offset < 0 {
		return errors.New("offset must not be negative")
	}
	result, err := s.db.ExecContext(ctx, "UPDATE log_files SET offset_bytes = ? WHERE id = ?", offset, fileID)
	if err != nil {
		return fmt.Errorf("update log offset: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update log offset: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("log file not found: %d", fileID)
	}
	return nil
}

func (s Store) refreshFile(ctx context.Context, watchedLogID int64, path string) (FileState, error) {
	stat, err := statFile(path)
	if err != nil {
		return FileState{}, err
	}

	current, err := s.getFileByPath(ctx, watchedLogID, path)
	if errors.Is(err, sql.ErrNoRows) {
		return s.insertFile(ctx, watchedLogID, stat, RotationNew)
	}
	if err != nil {
		return FileState{}, err
	}

	rotation := RotationNone
	offset := current.OffsetBytes
	if current.Device != stat.Device || current.Inode != stat.Inode {
		rotation = RotationReplaced
		offset = 0
	} else if stat.SizeBytes < current.OffsetBytes {
		rotation = RotationTruncated
		offset = 0
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE log_files
SET device = ?, inode = ?, size_bytes = ?, offset_bytes = ?, last_seen_at = CURRENT_TIMESTAMP
WHERE id = ?;
`, stat.Device, stat.Inode, stat.SizeBytes, offset, current.ID)
	if err != nil {
		return FileState{}, fmt.Errorf("update log file state: %w", err)
	}

	stat.ID = current.ID
	stat.WatchedLogID = watchedLogID
	stat.OffsetBytes = offset
	stat.Rotation = rotation
	return stat, nil
}

func (s Store) insertFile(ctx context.Context, watchedLogID int64, stat FileState, rotation RotationState) (FileState, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO log_files(watched_log_id, path, device, inode, size_bytes, offset_bytes, last_seen_at)
VALUES(?, ?, ?, ?, ?, 0, CURRENT_TIMESTAMP);
`, watchedLogID, stat.Path, stat.Device, stat.Inode, stat.SizeBytes)
	if err != nil {
		return FileState{}, fmt.Errorf("insert log file state: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return FileState{}, fmt.Errorf("read log file id: %w", err)
	}
	stat.ID = id
	stat.WatchedLogID = watchedLogID
	stat.Rotation = rotation
	return stat, nil
}

func (s Store) getFileByPath(ctx context.Context, watchedLogID int64, path string) (FileState, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, watched_log_id, path, COALESCE(device, ''), COALESCE(inode, ''), size_bytes, offset_bytes
FROM log_files
WHERE watched_log_id = ? AND path = ?;
`, watchedLogID, path)
	return scanFileState(row)
}

func (s Store) missingFiles(ctx context.Context, watchedLogID int64, observed []string) ([]FileState, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, watched_log_id, path, COALESCE(device, ''), COALESCE(inode, ''), size_bytes, offset_bytes
FROM log_files
WHERE watched_log_id = ?;
`, watchedLogID)
	if err != nil {
		return nil, fmt.Errorf("list tracked log files: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]struct{}, len(observed))
	for _, path := range observed {
		seen[path] = struct{}{}
	}

	var missing []FileState
	for rows.Next() {
		state, err := scanFileState(rows)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[state.Path]; ok {
			continue
		}
		state.Rotation = RotationMissing
		missing = append(missing, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tracked log files: %w", err)
	}
	return missing, nil
}

func scanFileState(row rowScanner) (FileState, error) {
	var state FileState
	if err := row.Scan(&state.ID, &state.WatchedLogID, &state.Path, &state.Device, &state.Inode, &state.SizeBytes, &state.OffsetBytes); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FileState{}, err
		}
		return FileState{}, fmt.Errorf("scan log file state: %w", err)
	}
	state.Rotation = RotationNone
	return state, nil
}

func expandPath(path string) ([]string, error) {
	if !hasGlobMeta(path) {
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			return nil, fmt.Errorf("stat log path: %w", err)
		}
		return []string{filepath.Clean(path)}, nil
	}

	matches, err := filepath.Glob(path)
	if err != nil {
		return nil, fmt.Errorf("expand log glob: %w", err)
	}
	sort.Strings(matches)
	return matches, nil
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func statFile(path string) (FileState, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileState{}, fmt.Errorf("stat log file: %w", err)
	}
	if info.IsDir() {
		return FileState{}, fmt.Errorf("log path is a directory: %s", path)
	}

	device, inode := fileIdentity(info)
	return FileState{
		Path:      filepath.Clean(path),
		Device:    device,
		Inode:     inode,
		SizeBytes: info.Size(),
	}, nil
}

func fileIdentity(info os.FileInfo) (string, string) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", ""
	}
	return strconv.FormatUint(uint64(stat.Dev), 10), strconv.FormatUint(uint64(stat.Ino), 10)
}
