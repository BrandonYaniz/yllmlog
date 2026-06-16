package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/BrandonYaniz/yllmlog/internal/socket"
)

func TestRunVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatal("version output is empty")
	}
}

func TestRunStatus(t *testing.T) {
	socketPath := t.TempDir() + "/yllmlog.sock"
	server, err := socket.NewServer(socketPath, func(_ context.Context, request socket.Request) (any, error) {
		if request.Action != socket.ActionStatus {
			t.Fatalf("Action = %q, want %q", request.Action, socket.ActionStatus)
		}
		return socket.Status{Version: "test", Ready: true}, nil
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() {
		server.Close()
	})

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--socket", socketPath, "status"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "yllmlogd test ready=true" {
		t.Fatalf("status output = %q", got)
	}
}
