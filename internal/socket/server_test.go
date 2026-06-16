package socket

import (
	"context"
	"encoding/json"
	"testing"
)

func TestServerStatus(t *testing.T) {
	socketPath := t.TempDir() + "/yllmlog.sock"
	server, err := NewServer(socketPath, func(_ context.Context, request Request) (any, error) {
		if request.Action != ActionStatus {
			t.Fatalf("Action = %q, want %q", request.Action, ActionStatus)
		}
		return Status{Version: "test", Ready: true}, nil
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

	response, err := Do(context.Background(), socketPath, Request{ID: "1", Action: ActionStatus})
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if !response.OK {
		t.Fatalf("response not OK: %s", response.Error)
	}
	if response.ID != "1" {
		t.Fatalf("response ID = %q, want 1", response.ID)
	}

	var status Status
	if err := json.Unmarshal(response.Result, &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Version != "test" || !status.Ready {
		t.Fatalf("status = %+v", status)
	}
}
