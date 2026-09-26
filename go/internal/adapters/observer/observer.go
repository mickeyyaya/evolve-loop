// Package observer is the per-phase stall detector that `evolve loop` runs
// beside every phase: it watches the phase's output for progress and writes
// NDJSON events. See docs/architecture/packages/internal-adapters-observer.md.
package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/observerengine"
)

// Default observer timings; policy.ObserverPolicy supplies the live values.
const (
	// DefaultPollS is the interval between progress checks.
	DefaultPollS = 5 * time.Second
	// DefaultStallS is how long a phase may show no progress before Watch emits stall_no_output.
	// See ADR-0030.
	DefaultStallS = 600 * time.Second
	// DefaultNudgeS is the soft-stall nudge threshold. The auto-spawn path sends
	// no nudges yet; the constant waits for the engine fold.
	// See ADR-0023.
	DefaultNudgeS = 300 * time.Second

	// observerEventsSuffix is excluded from the activity scan so the observer's
	// own writes never reset the stall clock.
	observerEventsSuffix = observerengine.EventsSuffix

	// activityScanMaxFiles bounds the per-poll workspace walk against a pathological tree.
	activityScanMaxFiles = 500
)

// Event is one observer emission, NDJSON-serialized.
type Event struct {
	TS       string `json:"ts"`
	Type     string `json:"type"`     // started | stall_no_output | stall_probe_active | stopped
	Severity string `json:"severity"` // info | incident
	Cycle    int    `json:"cycle"`
	Phase    string `json:"phase"`
	Agent    string `json:"agent"`
	Reason   string `json:"reason,omitempty"`
}

// Config pins the observer's runtime parameters.
type Config struct {
	StallS    time.Duration
	PollS     time.Duration
	Cycle     int
	Phase     string
	Agent     string
	StdoutLog string
	// WorkspaceDir, when set, makes a fresh file write anywhere under it count as
	// progress; a tmux driver's output reaches the stdout log only at exit.
	WorkspaceDir string

	// LivenessProbe, when set, is consulted only at the stall threshold; true
	// resets the stall clock and emits stall_probe_active instead of stall_no_output.
	LivenessProbe func() bool

	// OnEvent, when set, receives every event synchronously after the sink write
	// and outside the sink lock. It must not block.
	OnEvent func(Event)
}

// Observer watches one phase's stdout log and workspace and emits events when rules fire.
type Observer struct {
	cfg     Config
	sink    io.Writer
	quit    chan struct{}
	once    sync.Once
	encMu   sync.Mutex // serializes sink writes
	nowFunc func() time.Time
}

// New constructs an Observer; a zero StallS or PollS takes the package default.
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

// Watch blocks until ctx is canceled or Stop is called. It returns ctx.Err()
// on cancellation and nil on Stop; a stall emits stall_no_output but never ends the watch.
func (o *Observer) Watch(ctx context.Context) error {
	o.emit("started", "info", "observer attached")
	lastGrowth := o.nowFunc()
	lastSize := o.statSize()
	lastActivity := o.newestActivity()
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
			act := o.newestActivity()
			if sz > lastSize || act.After(lastActivity) {
				lastSize = sz
				if act.After(lastActivity) {
					lastActivity = act
				}
				lastGrowth = o.nowFunc()
				stallEmitted = false
				continue
			}
			if !stallEmitted && o.nowFunc().Sub(lastGrowth) >= o.cfg.StallS {
				if o.cfg.LivenessProbe != nil && o.cfg.LivenessProbe() {
					lastGrowth = o.nowFunc()
					o.emit("stall_probe_active", "info",
						fmt.Sprintf("no fs growth for %s but liveness probe active", o.cfg.StallS))
					continue
				}
				o.emit("stall_no_output", "incident",
					fmt.Sprintf("no stdout growth for %s", o.cfg.StallS))
				stallEmitted = true
			}
		}
	}
}

// newestActivity returns the newest file mtime under WorkspaceDir, excluding
// the observer's own events file. The zero Time means no signal.
func (o *Observer) newestActivity() time.Time {
	if o.cfg.WorkspaceDir == "" {
		return time.Time{}
	}
	var newest time.Time
	seen := 0
	_ = filepath.Walk(o.cfg.WorkspaceDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // an unreadable entry is skipped, never fatal
		}
		if info.IsDir() {
			return nil
		}
		// Skipped before the cap is charged: the observer's own sink is not work.
		if strings.HasSuffix(info.Name(), observerEventsSuffix) {
			return nil
		}
		if seen >= activityScanMaxFiles {
			return filepath.SkipAll
		}
		seen++
		if mt := info.ModTime(); mt.After(newest) {
			newest = mt
		}
		return nil
	})
	return newest
}

// Stop signals the Watch goroutine to exit. Idempotent.
func (o *Observer) Stop() error {
	o.once.Do(func() { close(o.quit) })
	return nil
}

// statSize returns the stdout log size, or 0 when it is unset or unreadable.
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
	o.encMu.Lock()
	if b, err := json.Marshal(e); err == nil {
		_, _ = o.sink.Write(append(b, '\n'))
	}
	o.encMu.Unlock()
	// Outside the lock, so a slow subscriber cannot hold up other emits.
	if o.cfg.OnEvent != nil {
		o.cfg.OnEvent(e)
	}
}
