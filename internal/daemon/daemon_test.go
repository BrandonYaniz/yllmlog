package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/BrandonYaniz/yllmlog/internal/config"
	"github.com/BrandonYaniz/yllmlog/internal/logs"
	"github.com/BrandonYaniz/yllmlog/internal/socket"
	"github.com/BrandonYaniz/yllmlog/internal/system"
	"github.com/BrandonYaniz/yllmlog/migrations"
)

func TestDaemonStatus(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.DefaultForPaths(system.Paths{
		DataDir:      tempDir + "/data",
		DaemonSocket: tempDir + "/run/yllmlog.sock",
		YLLMDSocket:  tempDir + "/run/yllmd.sock",
		YLLMDProfile: "phi",
	})
	migrations := fstest.MapFS{
		"migrations/001_initial.sql": {Data: []byte(`CREATE TABLE example (id INTEGER PRIMARY KEY);`)},
	}

	daemon, err := New(context.Background(), cfg, migrations, "migrations")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := daemon.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() {
		daemon.Close()
	})

	response, err := socket.Do(context.Background(), cfg.Daemon.Socket, socket.Request{Action: socket.ActionStatus})
	if err != nil {
		t.Fatalf("socket Do returned error: %v", err)
	}
	status, err := socket.DecodeResult[socket.Status](response)
	if err != nil {
		t.Fatalf("DecodeResult returned error: %v", err)
	}
	if !status.Ready {
		t.Fatal("daemon status is not ready")
	}
	if status.Version == "" {
		t.Fatal("daemon status version is empty")
	}
}

func TestDaemonProcessesWatchedLog(t *testing.T) {
	tempDir := t.TempDir()
	socketDir, err := os.MkdirTemp("/tmp", "yllmlog-daemon-")
	if err != nil {
		t.Fatalf("create socket temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(socketDir) })
	logPath := filepath.Join(tempDir, "messages")
	if err := os.WriteFile(logPath, []byte("disk error 42\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
	cfg := config.DefaultForPaths(system.Paths{
		DataDir:      filepath.Join(tempDir, "data"),
		DaemonSocket: filepath.Join(socketDir, "yllmlog.sock"),
		YLLMDSocket:  filepath.Join(socketDir, "yllmd.sock"),
		YLLMDProfile: "phi",
	})

	daemon, err := New(context.Background(), cfg, migrations.FS, ".")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	daemon.pollInterval = 10 * time.Millisecond
	if _, err := daemon.db.Exec(`
INSERT INTO watched_logs(path, service_name) VALUES(?, 'system');
INSERT INTO rules(name, source, matcher, pattern, action, priority, enabled)
VALUES('disk errors', 'system_default', 'contains', 'disk error', 'escalate', 10, 1);
`, logPath); err != nil {
		daemon.Close()
		t.Fatalf("seed database: %v", err)
	}
	if err := daemon.Listen(); err != nil {
		daemon.Close()
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { daemon.Close() })

	deadline := time.Now().Add(2 * time.Second)
	for {
		var count int
		if err := daemon.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count); err != nil {
			t.Fatalf("count events: %v", err)
		}
		if count == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("daemon did not process watched log before deadline")
		}
		time.Sleep(10 * time.Millisecond)
	}

	response, err := socket.Do(context.Background(), cfg.Daemon.Socket, socket.Request{Action: socket.ActionStatus})
	if err != nil {
		t.Fatalf("status request returned error: %v", err)
	}
	status, err := socket.DecodeResult[socket.Status](response)
	if err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.LastCycleAt == "" || status.LastCycleError != "" {
		t.Fatalf("status = %+v", status)
	}
}

func TestDaemonWatchedLogs(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.DefaultForPaths(system.Paths{
		DataDir:      tempDir + "/data",
		DaemonSocket: tempDir + "/run/yllmlog.sock",
		YLLMDSocket:  tempDir + "/run/yllmd.sock",
		YLLMDProfile: "phi",
	})

	daemon, err := New(context.Background(), cfg, fstest.MapFS{
		"migrations/001_initial.sql": {Data: []byte(`
CREATE TABLE watched_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    service_name TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`)},
	}, "migrations")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := daemon.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() {
		daemon.Close()
	})

	addParams, err := json.Marshal(socket.LogsAddParams{Path: "/var/log/messages", ServiceName: "system"})
	if err != nil {
		t.Fatalf("marshal add params: %v", err)
	}
	response, err := socket.Do(context.Background(), cfg.Daemon.Socket, socket.Request{
		Action: socket.ActionLogsAdd,
		Params: addParams,
	})
	if err != nil {
		t.Fatalf("logs.add returned error: %v", err)
	}
	added, err := socket.DecodeResult[logs.WatchedLog](response)
	if err != nil {
		t.Fatalf("decode logs.add: %v", err)
	}
	if added.Path != "/var/log/messages" {
		t.Fatalf("added path = %q", added.Path)
	}

	response, err = socket.Do(context.Background(), cfg.Daemon.Socket, socket.Request{Action: socket.ActionLogsList})
	if err != nil {
		t.Fatalf("logs.list returned error: %v", err)
	}
	watched, err := socket.DecodeResult[[]logs.WatchedLog](response)
	if err != nil {
		t.Fatalf("decode logs.list: %v", err)
	}
	if len(watched) != 1 {
		t.Fatalf("len(watched) = %d, want 1", len(watched))
	}

	removeParams, err := json.Marshal(socket.LogsRemoveParams{Path: "/var/log/messages"})
	if err != nil {
		t.Fatalf("marshal remove params: %v", err)
	}
	response, err = socket.Do(context.Background(), cfg.Daemon.Socket, socket.Request{
		Action: socket.ActionLogsRemove,
		Params: removeParams,
	})
	if err != nil {
		t.Fatalf("logs.remove returned error: %v", err)
	}
	if !response.OK {
		t.Fatalf("logs.remove not OK: %s", response.Error)
	}
}
