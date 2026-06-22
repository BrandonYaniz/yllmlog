package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/BrandonYaniz/yllmlog/internal/events"
	"github.com/BrandonYaniz/yllmlog/internal/logs"
	"github.com/BrandonYaniz/yllmlog/internal/rules"
)

type CycleResult struct {
	WatchedLogs int
	Files       int
	Lines       int
	Matched     int
	Ignored     int
	Events      int
}

type Processor struct {
	db     *sql.DB
	logs   logs.Store
	events events.Store
	now    func() time.Time
}

func NewProcessor(db *sql.DB) Processor {
	return Processor{
		db:     db,
		logs:   logs.NewStore(db),
		events: events.NewStore(db),
		now:    time.Now,
	}
}

func (p Processor) RunCycle(ctx context.Context) (CycleResult, error) {
	watchedLogs, err := p.logs.List(ctx)
	if err != nil {
		return CycleResult{}, err
	}
	loadedRules, err := rules.LoadEnabled(ctx, p.db)
	if err != nil {
		return CycleResult{}, err
	}
	if len(loadedRules) == 0 {
		return CycleResult{WatchedLogs: len(watchedLogs)}, nil
	}
	engine, err := rules.NewEngine(loadedRules)
	if err != nil {
		return CycleResult{}, fmt.Errorf("build rule engine: %w", err)
	}

	result := CycleResult{WatchedLogs: len(watchedLogs)}
	for _, watched := range watchedLogs {
		if !watched.Enabled {
			continue
		}
		files, err := p.logs.RefreshFiles(ctx, watched)
		if err != nil {
			return result, fmt.Errorf("refresh watched log %q: %w", watched.Path, err)
		}
		for _, file := range files {
			if file.Rotation == logs.RotationMissing {
				continue
			}
			result.Files++
			if err := p.processFile(ctx, watched, file, engine, &result); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func (p Processor) processFile(ctx context.Context, watched logs.WatchedLog, file logs.FileState, engine rules.Engine, result *CycleResult) error {
	lines, err := logs.ReadCompleteLines(file)
	if err != nil {
		return fmt.Errorf("read log file %q: %w", file.Path, err)
	}

	for _, line := range lines {
		result.Lines++
		match, matched := engine.Match(rules.LineContext{
			Line:    line.Text,
			Service: watched.ServiceName,
		})
		if !matched {
			if err := p.logs.UpdateOffset(ctx, file.ID, line.EndOffset); err != nil {
				return err
			}
			continue
		}

		result.Matched++
		if err := rules.RecordMatch(ctx, p.db, match.Rule.ID); err != nil {
			return err
		}
		if match.Action == rules.ActionIgnore {
			result.Ignored++
			if err := p.logs.UpdateOffset(ctx, file.ID, line.EndOffset); err != nil {
				return err
			}
			continue
		}

		if _, err := p.events.Record(ctx, events.Occurrence{
			LogFileID: file.ID,
			Service:   watched.ServiceName,
			Severity:  severityForAction(match.Action),
			Summary:   strings.TrimSpace(line.Text),
			Line:      line.Text,
			At:        p.now(),
		}); err != nil {
			return fmt.Errorf("record event for %q: %w", file.Path, err)
		}
		result.Events++
		if err := p.logs.UpdateOffset(ctx, file.ID, line.EndOffset); err != nil {
			return err
		}
	}
	return nil
}

func severityForAction(action rules.Action) string {
	switch action {
	case rules.ActionEscalate:
		return "critical"
	case rules.ActionReport:
		return "warning"
	case rules.ActionAnalyze:
		return "error"
	default:
		return "unknown"
	}
}
