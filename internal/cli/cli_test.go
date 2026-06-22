package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/BrandonYaniz/yllmlog/internal/events"
	"github.com/BrandonYaniz/yllmlog/internal/logs"
	"github.com/BrandonYaniz/yllmlog/internal/reports"
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

func TestRunReportsCommand(t *testing.T) {
	socketPath := t.TempDir() + "/yllmlog.sock"
	server, err := socket.NewServer(socketPath, func(_ context.Context, request socket.Request) (any, error) {
		if request.Action != socket.ActionReportsGenerate {
			t.Fatalf("unexpected action %q", request.Action)
		}
		var params socket.ReportsGenerateParams
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return nil, err
		}
		if params.Kind != "daily" {
			t.Fatalf("report kind = %q", params.Kind)
		}
		return reports.Report{ID: 1, Kind: reports.KindDaily, Body: "daily body\n"}, nil
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { server.Close() })

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--socket", socketPath, "reports", "daily"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("reports exit code = %d, stderr = %s", code, stderr.String())
	}
	if stdout.String() != "daily body\n" {
		t.Fatalf("reports output = %q", stdout.String())
	}
}

func TestRunIssuesCommands(t *testing.T) {
	socketPath := t.TempDir() + "/yllmlog.sock"
	event := events.Event{ID: 7, Severity: "critical", ServiceName: "disk", Summary: "disk full", TotalOccurrences: 3, FirstSeenAt: "first", LastSeenAt: "last"}
	server, err := socket.NewServer(socketPath, func(_ context.Context, request socket.Request) (any, error) {
		switch request.Action {
		case socket.ActionIssuesList:
			return []events.Event{event}, nil
		case socket.ActionIssuesGet:
			return events.Detail{
				Event:       event,
				Occurrences: []events.OccurrenceDetail{{ID: 1, OccurredAt: "now", Line: "disk full 95"}},
			}, nil
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
	t.Cleanup(func() { server.Close() })

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"--socket", socketPath, "issues"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("issues exit code = %d, stderr = %s", code, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "7\tcritical\tdisk\t3\tdisk full" {
		t.Fatalf("issues output = %q", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"--socket", socketPath, "issues", "7"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("issues get exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Issue 7 [critical] disk full") || !strings.Contains(stdout.String(), "disk full 95") {
		t.Fatalf("issues get output = %q", stdout.String())
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
