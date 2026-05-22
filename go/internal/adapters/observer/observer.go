// Package observer ports the per-phase observer service from
// scripts/dispatch/phase-observer.sh — Phase 2 scope is the minimal
// stall-detection skeleton + NDJSON event emission. The full 5-rule
// detector (infinite_loop, error_spike, cost_anomaly, rate_limit,
// stall_no_output) is Phase 3 follow-up; only stall_no_output is
// implemented here.
//
// Wire-up: orchestrator constructs one Observer per phase, hands the
// goroutine the phase's stdout-log path + a writer that fans out to
// <phase>-observer-events.ndjson and slog. Watch() blocks until ctx
// cancellation, Stop(), or natural exit on stall.
package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Defaults from CLAUDE.md env-var table.
const (
	DefaultPollS  = 5 * time.Second
	DefaultStallS = 600 * time.Second
)

// Event is one observer emission, NDJSON-serialized.
type Event struct {
	TS       string `json:"ts"`
	Type     string `json:"type"`     // stall_no_output | started | stopped
	Severity string `json:"severity"` // info | warn | incident
	Cycle    int    `json:"cycle"`
	Phase    string `json:"phase"`
	Agent    string `json:"agent"`
	Reason   string `json:"reason,omitempty"`
}

// Config pins the observer's runtime parameters.
type Config struct {
	StallS    time.Duration // EVOLVE_OBSERVER_STALL_S
	PollS     time.Duration // EVOLVE_OBSERVER_POLL_S
	Cycle     int
	Phase     string
	Agent     string
	StdoutLog string // path to subagent stdout-log file
}

// Observer watches one phase's stdout-log for activity and emits
// events when rules fire.
type Observer struct {
	cfg     Config
	sink    io.Writer
	quit    chan struct{}
	once    sync.Once
	encMu   sync.Mutex // serialize NDJSON writes
	nowFunc func() time.Time
}

// New constructs an Observer. Zero values in Config get the
// CLAUDE.md defaults.
func New(cfg Config, sink io.Writer) *Observer {
	if cfg.StallS == 0 {
		cfg.StallS = DefaultStallS
	}
	if cfg.PollS == 0 {
		cfg.PollS = DefaultPollS
	}
	return &Observer{
		cfg:     cfg,
		sink:    sink,
		quit:    make(chan struct{}),
		nowFunc: time.Now,
	}
}

// Watch blocks until ctx cancels, Stop is called, or a terminal
// condition fires (none in Phase 2 — just stall events). Returns
// ctx.Err() on cancellation; nil on Stop or natural exit.
func (o *Observer) Watch(ctx context.Context) error {
	o.emit("started", "info", "observer attached")
	lastGrowth := o.nowFunc()
	lastSize := o.statSize()
	stallEmitted := false

	ticker := time.NewTicker(o.cfg.PollS)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			o.emit("stopped", "info", "context canceled")
			return ctx.Err()
		case <-o.quit:
			o.emit("stopped", "info", "stop requested")
			return nil
		case <-ticker.C:
			sz := o.statSize()
			if sz > lastSize {
				lastSize = sz
				lastGrowth = o.nowFunc()
				stallEmitted = false
				continue
			}
			if !stallEmitted && o.nowFunc().Sub(lastGrowth) >= o.cfg.StallS {
				o.emit("stall_no_output", "incident",
					fmt.Sprintf("no stdout growth for %s", o.cfg.StallS))
				stallEmitted = true
			}
		}
	}
}

// Stop signals the Watch goroutine to exit. Idempotent.
func (o *Observer) Stop() error {
	o.once.Do(func() { close(o.quit) })
	return nil
}

// statSize returns the current size of the stdout-log, or 0 on error
// (treat missing/inaccessible file as "no output"; ticker retries).
func (o *Observer) statSize() int64 {
	if o.cfg.StdoutLog == "" {
		return 0
	}
	info, err := os.Stat(o.cfg.StdoutLog)
	if err != nil {
		return 0
	}
	return info.Size()
}

// emit serializes an Event to the sink as one NDJSON line.
func (o *Observer) emit(eventType, severity, reason string) {
	e := Event{
		TS:       o.nowFunc().UTC().Format(time.RFC3339Nano),
		Type:     eventType,
		Severity: severity,
		Cycle:    o.cfg.Cycle,
		Phase:    o.cfg.Phase,
		Agent:    o.cfg.Agent,
		Reason:   reason,
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	o.encMu.Lock()
	defer o.encMu.Unlock()
	_, _ = o.sink.Write(append(b, '\n'))
}
