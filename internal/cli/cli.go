package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/BrandonYaniz/yllmlog/internal/config"
	"github.com/BrandonYaniz/yllmlog/internal/socket"
	"github.com/BrandonYaniz/yllmlog/internal/version"
)

// Run executes the yllmlog CLI.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("yllmlog", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to yllmlog config file")
	socketPath := flags.String("socket", "", "path to yllmlog daemon socket")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	command := "help"
	if flags.NArg() > 0 {
		command = flags.Arg(0)
	}

	switch command {
	case "version":
		fmt.Fprintln(stdout, version.Current())
		return 0
	case "status":
		cfg, err := loadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		targetSocket := cfg.Daemon.Socket
		if *socketPath != "" {
			targetSocket = *socketPath
		}
		return status(ctx, targetSocket, stdout, stderr)
	default:
		printUsage(stdout)
		return 0
	}
}

func loadConfig(path string) (config.Config, error) {
	if path == "" {
		return config.Default(), nil
	}
	return config.Load(path)
}

func status(ctx context.Context, socketPath string, stdout, stderr io.Writer) int {
	response, err := socket.Do(ctx, socketPath, socket.Request{Action: socket.ActionStatus})
	if err != nil {
		fmt.Fprintf(stderr, "status: %v\n", err)
		return 1
	}

	status, err := socket.DecodeResult[socket.Status](response)
	if err != nil {
		fmt.Fprintf(stderr, "status: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "yllmlogd %s ready=%t\n", status.Version, status.Ready)
	return 0
}

func printUsage(stdout io.Writer) {
	fmt.Fprintln(stdout, "Usage: yllmlog [--config path] [--socket path] <command>")
	fmt.Fprintln(stdout, "Commands:")
	fmt.Fprintln(stdout, "  status   Show daemon status")
	fmt.Fprintln(stdout, "  version  Print CLI version")
}
