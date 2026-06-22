package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/BrandonYaniz/yllmlog/internal/app"
	"github.com/BrandonYaniz/yllmlog/internal/config"
	"github.com/BrandonYaniz/yllmlog/internal/db"
	"github.com/BrandonYaniz/yllmlog/internal/events"
	"github.com/BrandonYaniz/yllmlog/internal/logs"
	"github.com/BrandonYaniz/yllmlog/internal/reports"
	"github.com/BrandonYaniz/yllmlog/internal/socket"
	"github.com/BrandonYaniz/yllmlog/internal/version"
)

// Daemon owns the local API server and persistent state.
type Daemon struct {
	cfg     config.Config
	db      *sql.DB
	logs    logs.Store
	events  events.Store
	reports reports.Generator
	process app.Processor
	server  *socket.Server

	pollInterval time.Duration
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	statusMu     sync.RWMutex
	lastCycleAt  time.Time
	lastCycleErr error
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
		cfg:          cfg,
		db:           database,
		logs:         logs.NewStore(database),
		events:       events.NewStore(database),
		reports:      reports.NewGenerator(database),
		process:      app.NewProcessor(database),
		pollInterval: 5 * time.Second,
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
	if err := d.server.Listen(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel
	d.wg.Add(1)
	go d.runProcessor(ctx)
	return nil
}

// Close stops the daemon and closes persistent resources.
func (d *Daemon) Close() error {
	var closeErr error
	if d.cancel != nil {
		d.cancel()
		d.wg.Wait()
	}
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
		return d.status(), nil
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
	case socket.ActionIssuesList:
		return d.events.List(ctx)
	case socket.ActionIssuesGet:
		var params socket.IssuesGetParams
		if err := decodeParams(request, &params); err != nil {
			return nil, err
		}
		return d.events.Get(ctx, params.ID)
	case socket.ActionReportsGenerate:
		var params socket.ReportsGenerateParams
		if err := decodeParams(request, &params); err != nil {
			return nil, err
		}
		return d.generateReport(ctx, reports.Kind(params.Kind))
	default:
		return nil, fmt.Errorf("unknown action %q", request.Action)
	}
}

func (d *Daemon) generateReport(ctx context.Context, kind reports.Kind) (reports.Report, error) {
	now := time.Now()
	switch kind {
	case reports.KindDaily:
		return d.reports.Generate(ctx, kind, now.Add(-24*time.Hour), now)
	case reports.KindWeekly:
		return d.reports.Generate(ctx, kind, now.Add(-7*24*time.Hour), now)
	default:
		return reports.Report{}, fmt.Errorf("unsupported report kind %q", kind)
	}
}

func (d *Daemon) runProcessor(ctx context.Context) {
	defer d.wg.Done()

	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := d.process.RunCycle(ctx)
			d.statusMu.Lock()
			d.lastCycleAt = time.Now()
			d.lastCycleErr = err
			d.statusMu.Unlock()
		}
	}
}

func (d *Daemon) status() socket.Status {
	d.statusMu.RLock()
	defer d.statusMu.RUnlock()

	status := socket.Status{
		Version: version.Current(),
		Ready:   true,
	}
	if !d.lastCycleAt.IsZero() {
		status.LastCycleAt = d.lastCycleAt.UTC().Format(time.RFC3339)
	}
	if d.lastCycleErr != nil {
		status.LastCycleError = d.lastCycleErr.Error()
	}
	return status
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
