package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/BrandonYaniz/yllmlog/internal/config"
	"github.com/BrandonYaniz/yllmlog/internal/daemon"
	"github.com/BrandonYaniz/yllmlog/internal/version"
	"github.com/BrandonYaniz/yllmlog/migrations"
)

func main() {
	configPath := flag.String("config", "", "path to yllmlog config file")
	flag.Parse()

	cfg := config.Default()
	if *configPath != "" {
		loaded, err := config.Load(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load config: %v\n", err)
			os.Exit(1)
		}
		cfg = loaded
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := daemon.New(ctx, cfg, migrations.FS, ".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "start daemon: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	if err := app.Listen(); err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("yllmlogd %s listening on %s\n", version.Current(), cfg.Daemon.Socket)
	<-ctx.Done()
}
