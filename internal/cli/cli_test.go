package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/BrandonYaniz/yllmlog/internal/logs"
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

func TestRunLogsCommands(t *testing.T) {
	socketPath := t.TempDir() + "/yllmlog.sock"
	var watched []logs.WatchedLog
	server, err := socket.NewServer(socketPath, func(_ context.Context, request socket.Request) (any, error) {
		switch request.Action {
		case socket.ActionLogsAdd:
			var params socket.LogsAddParams
			if err := json.Unmarshal(request.Params, &params); err != nil {
				return nil, err
			}
			watched = []logs.WatchedLog{{ID: 1, Path: params.Path, ServiceName: params.ServiceName, Enabled: true}}
			return watched[0], nil
		case socket.ActionLogsList:
			return watched, nil
		case socket.ActionLogsRemove:
			watched = nil
			return nil, nil
		default:
			t.Fatalf("unexpected action %q", request.Action)
			return nil, nil
		}
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
	code := Run(context.Background(), []string{"--socket", socketPath, "logs", "add", "--service", "system", "/var/log/messages"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("logs add exit code = %d, stderr = %s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "added /var/log/messages" {
		t.Fatalf("logs add output = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"--socket", socketPath, "logs", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("logs list exit code = %d, stderr = %s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "/var/log/messages\tsystem" {
		t.Fatalf("logs list output = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"--socket", socketPath, "logs", "remove", "/var/log/messages"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("logs remove exit code = %d, stderr = %s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "removed /var/log/messages" {
		t.Fatalf("logs remove output = %q", got)
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
