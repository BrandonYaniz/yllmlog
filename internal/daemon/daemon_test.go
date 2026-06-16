package daemon

import (
	"context"
	"encoding/json"
	"testing"
	"testing/fstest"

	"github.com/BrandonYaniz/yllmlog/internal/config"
	"github.com/BrandonYaniz/yllmlog/internal/logs"
	"github.com/BrandonYaniz/yllmlog/internal/socket"
	"github.com/BrandonYaniz/yllmlog/internal/system"
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
