package system

import "testing"

func TestDefaultPaths(t *testing.T) {
	tests := []struct {
		goos    string
		dataDir string
	}{
		{goos: "freebsd", dataDir: "/var/db/yllmlog"},
		{goos: "darwin", dataDir: "/Library/Application Support/yllmlog"},
		{goos: "linux", dataDir: "/var/lib/yllmlog"},
	}

	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			paths, err := DefaultPaths(tt.goos)
			if err != nil {
				t.Fatalf("DefaultPaths(%q) returned error: %v", tt.goos, err)
			}
			if paths.DataDir != tt.dataDir {
				t.Fatalf("DataDir = %q, want %q", paths.DataDir, tt.dataDir)
			}
			if paths.DaemonSocket != "/var/run/yllmlog/yllmlog.sock" {
				t.Fatalf("DaemonSocket = %q", paths.DaemonSocket)
			}
			if paths.YLLMDSocket != "/var/run/yllmd/yllmd.sock" {
				t.Fatalf("YLLMDSocket = %q", paths.YLLMDSocket)
			}
			if paths.YLLMDProfile != "phi" {
				t.Fatalf("YLLMDProfile = %q", paths.YLLMDProfile)
			}
		})
	}
}

func TestDefaultPathsRejectsUnsupportedOS(t *testing.T) {
	if _, err := DefaultPaths("plan9"); err == nil {
		t.Fatal("DefaultPaths accepted unsupported OS")
	}
}
