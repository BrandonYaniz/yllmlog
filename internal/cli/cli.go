package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/BrandonYaniz/yllmlog/internal/config"
	"github.com/BrandonYaniz/yllmlog/internal/events"
	"github.com/BrandonYaniz/yllmlog/internal/logs"
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
	case "logs":
		cfg, err := loadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		targetSocket := cfg.Daemon.Socket
		if *socketPath != "" {
			targetSocket = *socketPath
		}
		return logsCommand(ctx, targetSocket, flags.Args()[1:], stdout, stderr)
	case "issues":
		cfg, err := loadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		targetSocket := cfg.Daemon.Socket
		if *socketPath != "" {
			targetSocket = *socketPath
		}
		return issuesCommand(ctx, targetSocket, flags.Args()[1:], stdout, stderr)
	default:
		printUsage(stdout)
		return 0
	}
}

func issuesCommand(ctx context.Context, socketPath string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		response, err := socket.Do(ctx, socketPath, socket.Request{Action: socket.ActionIssuesList})
		if err != nil {
			fmt.Fprintf(stderr, "issues: %v\n", err)
			return 1
		}
		listed, err := socket.DecodeResult[[]events.Event](response)
		if err != nil {
			fmt.Fprintf(stderr, "issues: %v\n", err)
			return 1
		}
		for _, event := range listed {
			fmt.Fprintf(stdout, "%d\t%s\t%s\t%d\t%s\n", event.ID, event.Severity, event.ServiceName, event.TotalOccurrences, event.Summary)
		}
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "issues accepts at most one event id")
		return 2
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		fmt.Fprintln(stderr, "issue id must be a positive integer")
		return 2
	}
	params, err := json.Marshal(socket.IssuesGetParams{ID: id})
	if err != nil {
		fmt.Fprintf(stderr, "issues: %v\n", err)
		return 1
	}
	response, err := socket.Do(ctx, socketPath, socket.Request{Action: socket.ActionIssuesGet, Params: params})
	if err != nil {
		fmt.Fprintf(stderr, "issues: %v\n", err)
		return 1
	}
	detail, err := socket.DecodeResult[events.Detail](response)
	if err != nil {
		fmt.Fprintf(stderr, "issues: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Issue %d [%s] %s\n", detail.Event.ID, detail.Event.Severity, detail.Event.Summary)
	fmt.Fprintf(stdout, "Service: %s\nOccurrences: %d\nFirst seen: %s\nLast seen: %s\n", detail.Event.ServiceName, detail.Event.TotalOccurrences, detail.Event.FirstSeenAt, detail.Event.LastSeenAt)
	for _, occurrence := range detail.Occurrences {
		fmt.Fprintf(stdout, "%s\t%s\n", occurrence.OccurredAt, occurrence.Line)
	}
	return 0
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

func logsCommand(ctx context.Context, socketPath string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printLogsUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		return logsList(ctx, socketPath, stdout, stderr)
	case "add":
		addFlags := flag.NewFlagSet("yllmlog logs add", flag.ContinueOnError)
		addFlags.SetOutput(stderr)
		serviceName := addFlags.String("service", "", "service name for this log")
		if err := addFlags.Parse(args[1:]); err != nil {
			return 2
		}
		if addFlags.NArg() != 1 {
			fmt.Fprintln(stderr, "logs add requires exactly one path")
			return 2
		}
		return logsAdd(ctx, socketPath, addFlags.Arg(0), *serviceName, stdout, stderr)
	case "remove":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "logs remove requires exactly one path")
			return 2
		}
		return logsRemove(ctx, socketPath, args[1], stdout, stderr)
	default:
		printLogsUsage(stderr)
		return 2
	}
}

func logsList(ctx context.Context, socketPath string, stdout, stderr io.Writer) int {
	response, err := socket.Do(ctx, socketPath, socket.Request{Action: socket.ActionLogsList})
	if err != nil {
		fmt.Fprintf(stderr, "logs list: %v\n", err)
		return 1
	}
	watched, err := socket.DecodeResult[[]logs.WatchedLog](response)
	if err != nil {
		fmt.Fprintf(stderr, "logs list: %v\n", err)
		return 1
	}
	for _, log := range watched {
		if log.ServiceName == "" {
			fmt.Fprintln(stdout, log.Path)
			continue
		}
		fmt.Fprintf(stdout, "%s\t%s\n", log.Path, log.ServiceName)
	}
	return 0
}

func logsAdd(ctx context.Context, socketPath, path, serviceName string, stdout, stderr io.Writer) int {
	params, err := json.Marshal(socket.LogsAddParams{Path: path, ServiceName: serviceName})
	if err != nil {
		fmt.Fprintf(stderr, "logs add: %v\n", err)
		return 1
	}
	response, err := socket.Do(ctx, socketPath, socket.Request{Action: socket.ActionLogsAdd, Params: params})
	if err != nil {
		fmt.Fprintf(stderr, "logs add: %v\n", err)
		return 1
	}
	watched, err := socket.DecodeResult[logs.WatchedLog](response)
	if err != nil {
		fmt.Fprintf(stderr, "logs add: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "added %s\n", watched.Path)
	return 0
}

func logsRemove(ctx context.Context, socketPath, path string, stdout, stderr io.Writer) int {
	params, err := json.Marshal(socket.LogsRemoveParams{Path: path})
	if err != nil {
		fmt.Fprintf(stderr, "logs remove: %v\n", err)
		return 1
	}
	response, err := socket.Do(ctx, socketPath, socket.Request{Action: socket.ActionLogsRemove, Params: params})
	if err != nil {
		fmt.Fprintf(stderr, "logs remove: %v\n", err)
		return 1
	}
	if !response.OK {
		fmt.Fprintf(stderr, "logs remove: %s\n", response.Error)
		return 1
	}
	fmt.Fprintf(stdout, "removed %s\n", path)
	return 0
}

func printUsage(stdout io.Writer) {
	fmt.Fprintln(stdout, "Usage: yllmlog [--config path] [--socket path] <command>")
	fmt.Fprintln(stdout, "Commands:")
	fmt.Fprintln(stdout, "  issues   List issues or show one issue")
	fmt.Fprintln(stdout, "  logs     Manage watched logs")
	fmt.Fprintln(stdout, "  status   Show daemon status")
	fmt.Fprintln(stdout, "  version  Print CLI version")
}

func printLogsUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage: yllmlog logs <list|add|remove>")
}
