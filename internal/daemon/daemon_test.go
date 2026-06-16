package daemon

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/BrandonYaniz/yllmlog/internal/config"
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
