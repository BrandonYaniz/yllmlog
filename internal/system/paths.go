package system

import "fmt"

const (
	defaultProfile = "phi"
	yllmdSocket    = "/var/run/yllmd/yllmd.sock"
)

// Paths contains filesystem locations that vary by operating system.
type Paths struct {
	DataDir      string
	DaemonSocket string
	YLLMDSocket  string
	YLLMDProfile string
}

// DefaultPaths returns yllmlog's default paths for a Go runtime OS name.
func DefaultPaths(goos string) (Paths, error) {
	switch goos {
	case "freebsd":
		return Paths{
			DataDir:      "/var/db/yllmlog",
			DaemonSocket: "/var/run/yllmlog/yllmlog.sock",
			YLLMDSocket:  yllmdSocket,
			YLLMDProfile: defaultProfile,
		}, nil
	case "darwin":
		return Paths{
			DataDir:      "/Library/Application Support/yllmlog",
			DaemonSocket: "/var/run/yllmlog/yllmlog.sock",
			YLLMDSocket:  yllmdSocket,
			YLLMDProfile: defaultProfile,
		}, nil
	case "linux":
		return Paths{
			DataDir:      "/var/lib/yllmlog",
			DaemonSocket: "/var/run/yllmlog/yllmlog.sock",
			YLLMDSocket:  yllmdSocket,
			YLLMDProfile: defaultProfile,
		}, nil
	default:
		return Paths{}, fmt.Errorf("unsupported operating system %q", goos)
	}
}
