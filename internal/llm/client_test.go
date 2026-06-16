package llm

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestClientGenerateWithMock(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "yllmd.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() {
		listener.Close()
		os.Remove(socketPath)
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		var request Request
		if err := json.NewDecoder(conn).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if request.Profile != "phi" {
			t.Errorf("Profile = %q, want phi", request.Profile)
		}
		if !request.JSON {
			t.Error("JSON mode was false")
		}
		if err := json.NewEncoder(conn).Encode(Response{
			JSON: json.RawMessage(`{"summary":"disk full","severity":"warning"}`),
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}()

	client, err := NewClient(socketPath, "phi")
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	response, err := client.Generate(context.Background(), "explain this", true)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	<-done

	var parsed struct {
		Summary  string `json:"summary"`
		Severity string `json:"severity"`
	}
	parsed, err = StrictJSON[struct {
		Summary  string `json:"summary"`
		Severity string `json:"severity"`
	}](response)
	if err != nil {
		t.Fatalf("StrictJSON returned error: %v", err)
	}
	if parsed.Summary != "disk full" || parsed.Severity != "warning" {
		t.Fatalf("parsed response = %+v", parsed)
	}
}

func TestStrictJSONRejectsUnknownFields(t *testing.T) {
	type payload struct {
		Summary string `json:"summary"`
	}

	_, err := StrictJSON[payload](Response{Text: `{"summary":"ok","extra":true}`})
	if err == nil {
		t.Fatal("StrictJSON accepted unknown field")
	}
}
