package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BrandonYaniz/yllmlog/internal/system"
)

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, `
data_dir: /tmp/yllmlog-data
yllmd:
  socket: /tmp/yllmd.sock
  profile: llama
daemon:
  socket: /tmp/yllmlog.sock
safety:
  allow_chat_mutations: false
  require_confirmation_for_risky_changes: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.DataDir != "/tmp/yllmlog-data" {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.YLLMD.Socket != "/tmp/yllmd.sock" {
		t.Fatalf("YLLMD.Socket = %q", cfg.YLLMD.Socket)
	}
	if cfg.YLLMD.Profile != "llama" {
		t.Fatalf("YLLMD.Profile = %q", cfg.YLLMD.Profile)
	}
	if cfg.Daemon.Socket != "/tmp/yllmlog.sock" {
		t.Fatalf("Daemon.Socket = %q", cfg.Daemon.Socket)
	}
	if cfg.Safety.AllowChatMutations {
		t.Fatal("AllowChatMutations = true, want false")
	}
	if !cfg.Safety.RequireConfirmationForRiskyChanges {
		t.Fatal("RequireConfirmationForRiskyChanges = false, want true")
	}
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	path := writeConfig(t, `
data_dir: /tmp/yllmlog-data
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.YLLMD.Socket == "" {
		t.Fatal("YLLMD.Socket was not defaulted")
	}
	if cfg.YLLMD.Profile != "phi" {
		t.Fatalf("YLLMD.Profile = %q, want phi", cfg.YLLMD.Profile)
	}
	if cfg.Daemon.Socket == "" {
		t.Fatal("Daemon.Socket was not defaulted")
	}
	if !cfg.Safety.AllowChatMutations {
		t.Fatal("AllowChatMutations = false, want default true")
	}
	if !cfg.Safety.RequireConfirmationForRiskyChanges {
		t.Fatal("RequireConfirmationForRiskyChanges = false, want default true")
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := writeConfig(t, `
data_dir: /tmp/yllmlog-data
network:
  listen: 127.0.0.1:9000
`)

	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted unknown field")
	}
}

func TestValidateRejectsRelativePaths(t *testing.T) {
	cfg := DefaultForPaths(system.Paths{
		DataDir:      "relative",
		DaemonSocket: "/tmp/yllmlog.sock",
		YLLMDSocket:  "/tmp/yllmd.sock",
		YLLMDProfile: "phi",
	})

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted relative data_dir")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "yllmlog.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
