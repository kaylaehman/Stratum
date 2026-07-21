package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/webhooks"
)

// Handler is a function that performs one automation run. It should return a
// human-readable detail string (e.g. "restarted 2 containers") and any error.
// A returned error marks the run as "error" and fires a notification.
type Handler func(ctx context.Context) (detail string, err error)

// View is the merged catalog + DB-row representation returned by the API.
type View struct {
	Key             string         `json:"key"`
	Label           string         `json:"label"`
	Description     string         `json:"description"`
	Category        string         `json:"category"`
	Enabled         bool           `json:"enabled"`
	IntervalSeconds int            `json:"interval_seconds"`
	Config          map[string]any `json:"config"`
	LastRun         *time.Time     `json:"last_run,omitempty"`
	LastStatus      string         `json:"last_status"`
	LastDetail      string         `json:"last_detail"`
}

// Engine drives the automation tick loop.
type Engine struct {
	store     db.Store
	actStore  *activity.Store
	webhooks  *webhooks.Dispatcher
	handlers  map[string]Handler
	logger    *slog.Logger
}

// New builds an Engine. handlers must map every catalog key to a handler;
// missing keys are silently skipped in the run loop (never panic).
func New(
	store db.Store,
	actStore *activity.Store,
	dispatcher *webhooks.Dispatcher,
	handlers map[string]Handler,
	logger *slog.Logger,
) *Engine {
	return &Engine{
		store:    store,
		actStore: actStore,
		webhooks: dispatcher,
		handlers: handlers,
		logger:   logger,
	}
}

// Run is the background tick loop. Cancels cleanly on ctx done.
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

// RunNow immediately executes the automation for the given key (manual trigger).
func (e *Engine) RunNow(ctx context.Context, key string) (string, error) {
	row, err := e.resolveRow(ctx, key)
	if err != nil {
		return "", err
	}
	return e.execute(ctx, key, row)
}

// ListViews merges catalog entries with DB overrides.
func (e *Engine) ListViews(ctx context.Context) ([]View, error) {
	rows, err := e.store.ListAutomations(ctx)
	if err != nil {
		return nil, err
	}
	dbMap := make(map[string]db.AutomationRow, len(rows))
	for _, r := range rows {
		dbMap[r.Key] = r
	}

	views := make([]View, 0, len(catalog))
	for _, ent := range catalog {
		v := entryToView(ent, dbMap[ent.Key])
		views = append(views, v)
	}
	return views, nil
}

// GetView returns a single merged view.
func (e *Engine) GetView(ctx context.Context, key string) (View, error) {
	ent, ok := CatalogEntry(key)
	if !ok {
		return View{}, fmt.Errorf("automation: unknown key %q", key)
	}
	row, _ := e.store.GetAutomation(ctx, key) // ErrNotFound => zero row is fine
	return entryToView(ent, row), nil
}

// tick runs one pass over all enabled automations that are due.
func (e *Engine) tick(ctx context.Context) {
	for _, ent := range catalog {
		if ctx.Err() != nil {
			return
		}
		row, err := e.resolveRow(ctx, ent.Key)
		if err != nil {
			e.logger.Error("automation: resolve row", "key", ent.Key, "error", err)
			continue
		}
		if !row.Enabled {
			continue
		}
		if !isDue(row) {
			continue
		}
		if _, err := e.execute(ctx, ent.Key, row); err != nil {
			// error already recorded inside execute; just log at debug
			e.logger.Debug("automation tick error", "key", ent.Key, "error", err)
		}
	}
}

// resolveRow returns the effective DB row for key (zero-valued with catalog
// defaults when no DB row exists).
func (e *Engine) resolveRow(ctx context.Context, key string) (db.AutomationRow, error) {
	ent, ok := CatalogEntry(key)
	if !ok {
		return db.AutomationRow{}, fmt.Errorf("automation: unknown key %q", key)
	}
	row, err := e.store.GetAutomation(ctx, key)
	if err != nil {
		// No override stored; seed a zero row with catalog defaults.
		row = db.AutomationRow{
			Key:             key,
			Enabled:         false,
			IntervalSeconds: ent.DefaultIntervalSeconds,
			ConfigJSON:      "{}",
		}
	}
	return row, nil
}

// isDue returns true when last_run + interval has elapsed (or has never run).
func isDue(row db.AutomationRow) bool {
	if row.LastRun == nil {
		return true
	}
	interval := time.Duration(row.IntervalSeconds) * time.Second
	return time.Since(*row.LastRun) >= interval
}

// execute sets status "running", runs the handler, and persists the result.
func (e *Engine) execute(ctx context.Context, key string, row db.AutomationRow) (string, error) {
	// Ensure row exists in DB so SetAutomationRun can UPDATE it.
	_ = e.store.UpsertAutomation(ctx, key, row.Enabled, row.IntervalSeconds, row.ConfigJSON)
	_ = e.store.SetAutomationRun(ctx, key, "running", "", time.Now())

	handler, ok := e.handlers[key]
	if !ok {
		detail := "no handler registered"
		_ = e.store.SetAutomationRun(ctx, key, "error", detail, time.Now())
		return detail, fmt.Errorf("automation: %s", detail)
	}

	detail, runErr := handler(ctx)
	status := "ok"
	if runErr != nil {
		status = "error"
		if detail == "" {
			detail = runErr.Error()
		}
	}

	ranAt := time.Now()
	_ = e.store.SetAutomationRun(ctx, key, status, detail, ranAt)

	// Audit the run.
	_ = e.actStore.Append(ctx, activity.Entry{
		Action:     activity.ActionAutomationRun,
		TargetType: strPtr(activity.TargetAutomation),
		TargetID:   strPtr(key),
		Detail:     map[string]any{"status": status, "detail": detail},
		Result:     resultString(runErr),
	})

	// Notify on error.
	if runErr != nil && e.webhooks != nil {
		e.webhooks.Notify(ctx, "automation.error", webhooks.Message{
			Title: fmt.Sprintf("Automation '%s' failed", key),
			Text:  detail,
		})
	}

	return detail, runErr
}

// entryToView merges a catalog entry with its DB row.
func entryToView(ent Entry, row db.AutomationRow) View {
	v := View{
		Key:             ent.Key,
		Label:           ent.Label,
		Description:     ent.Description,
		Category:        ent.Category,
		Enabled:         row.Enabled,
		IntervalSeconds: ent.DefaultIntervalSeconds,
		Config:          copyConfig(ent.DefaultConfig),
		LastStatus:      row.LastStatus,
		LastDetail:      row.LastDetail,
		LastRun:         row.LastRun,
	}
	if row.IntervalSeconds > 0 {
		v.IntervalSeconds = row.IntervalSeconds
	}
	if row.ConfigJSON != "" && row.ConfigJSON != "{}" {
		var override map[string]any
		if err := json.Unmarshal([]byte(row.ConfigJSON), &override); err == nil {
			for k, val := range override {
				v.Config[k] = val
			}
		}
	}
	return v
}

func copyConfig(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func strPtr(s string) *string { return &s }

func resultString(err error) string {
	if err != nil {
		return activity.ResultError
	}
	return activity.ResultSuccess
}
