package logs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// WatchedLog is one configured log path or glob.
type WatchedLog struct {
	ID          int64  `json:"id"`
	Path        string `json:"path"`
	ServiceName string `json:"service_name,omitempty"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Store persists watched log settings.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) Store {
	return Store{db: db}
}

func (s Store) Add(ctx context.Context, path, serviceName string) (WatchedLog, error) {
	normalizedPath, err := normalizePath(path)
	if err != nil {
		return WatchedLog{}, err
	}
	serviceName = strings.TrimSpace(serviceName)

	result, err := s.db.ExecContext(ctx, `
INSERT INTO watched_logs(path, service_name, enabled, updated_at)
VALUES(?, NULLIF(?, ''), 1, CURRENT_TIMESTAMP)
ON CONFLICT(path) DO UPDATE SET
    service_name = excluded.service_name,
    enabled = 1,
    updated_at = CURRENT_TIMESTAMP;
`, normalizedPath, serviceName)
	if err != nil {
		return WatchedLog{}, fmt.Errorf("add watched log: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil && id > 0 {
		return s.Get(ctx, id)
	}
	return s.GetByPath(ctx, normalizedPath)
}

func (s Store) List(ctx context.Context) ([]WatchedLog, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, path, COALESCE(service_name, ''), enabled, created_at, updated_at
FROM watched_logs
ORDER BY path;
`)
	if err != nil {
		return nil, fmt.Errorf("list watched logs: %w", err)
	}
	defer rows.Close()

	var watched []WatchedLog
	for rows.Next() {
		log, err := scanWatchedLog(rows)
		if err != nil {
			return nil, err
		}
		watched = append(watched, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list watched logs: %w", err)
	}
	return watched, nil
}

func (s Store) Remove(ctx context.Context, path string) error {
	normalizedPath, err := normalizePath(path)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM watched_logs WHERE path = ?", normalizedPath)
	if err != nil {
		return fmt.Errorf("remove watched log: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("remove watched log: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("watched log not found: %s", normalizedPath)
	}
	return nil
}

func (s Store) Get(ctx context.Context, id int64) (WatchedLog, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, path, COALESCE(service_name, ''), enabled, created_at, updated_at
FROM watched_logs
WHERE id = ?;
`, id)
	return scanWatchedLog(row)
}

func (s Store) GetByPath(ctx context.Context, path string) (WatchedLog, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, path, COALESCE(service_name, ''), enabled, created_at, updated_at
FROM watched_logs
WHERE path = ?;
`, path)
	return scanWatchedLog(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWatchedLog(row rowScanner) (WatchedLog, error) {
	var watched WatchedLog
	var enabled int
	if err := row.Scan(&watched.ID, &watched.Path, &watched.ServiceName, &enabled, &watched.CreatedAt, &watched.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return WatchedLog{}, err
		}
		return WatchedLog{}, fmt.Errorf("scan watched log: %w", err)
	}
	watched.Enabled = enabled != 0
	return watched, nil
}

func normalizePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("log path is required")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("log path must be absolute")
	}
	return filepath.Clean(path), nil
}
