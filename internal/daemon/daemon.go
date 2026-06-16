package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/BrandonYaniz/yllmlog/internal/config"
	"github.com/BrandonYaniz/yllmlog/internal/db"
	"github.com/BrandonYaniz/yllmlog/internal/logs"
	"github.com/BrandonYaniz/yllmlog/internal/socket"
	"github.com/BrandonYaniz/yllmlog/internal/version"
)

// Daemon owns the local API server and persistent state.
type Daemon struct {
	cfg    config.Config
	db     *sql.DB
	logs   logs.Store
	server *socket.Server
}

// New opens the database, applies migrations, and prepares the socket server.
func New(ctx context.Context, cfg config.Config, migrationsFS fs.FS, migrationsDir string) (*Daemon, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	database, err := db.Open(cfg.DataDir + "/yllmlog.db")
	if err != nil {
		return nil, err
	}
	if err := db.ApplyMigrations(ctx, database, migrationsFS, migrationsDir); err != nil {
		database.Close()
		return nil, err
	}

	daemon := &Daemon{
		cfg:  cfg,
		db:   database,
		logs: logs.NewStore(database),
	}
	server, err := socket.NewServer(cfg.Daemon.Socket, daemon.handle)
	if err != nil {
		database.Close()
		return nil, err
	}
	daemon.server = server
	return daemon, nil
}

// Listen starts accepting local socket requests.
func (d *Daemon) Listen() error {
	if d.server == nil {
		return errors.New("daemon server is not initialized")
	}
	return d.server.Listen()
}

// Close stops the daemon and closes persistent resources.
func (d *Daemon) Close() error {
	var closeErr error
	if d.server != nil {
		closeErr = d.server.Close()
	}
	if d.db != nil {
		if err := d.db.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (d *Daemon) handle(ctx context.Context, request socket.Request) (any, error) {
	switch request.Action {
	case socket.ActionStatus:
		return socket.Status{Version: version.Current(), Ready: true}, nil
	case socket.ActionLogsList:
		return d.logs.List(ctx)
	case socket.ActionLogsAdd:
		var params socket.LogsAddParams
		if err := decodeParams(request, &params); err != nil {
			return nil, err
		}
		return d.logs.Add(ctx, params.Path, params.ServiceName)
	case socket.ActionLogsRemove:
		var params socket.LogsRemoveParams
		if err := decodeParams(request, &params); err != nil {
			return nil, err
		}
		return nil, d.logs.Remove(ctx, params.Path)
	default:
		return nil, fmt.Errorf("unknown action %q", request.Action)
	}
}

func decodeParams(request socket.Request, target any) error {
	if len(request.Params) == 0 {
		return errors.New("request params are required")
	}
	if err := json.Unmarshal(request.Params, target); err != nil {
		return fmt.Errorf("decode request params: %w", err)
	}
	return nil
}
