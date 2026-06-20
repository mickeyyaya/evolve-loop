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
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Defaults from CLAUDE.md env-var table.
const (
	DefaultPollS = 5 * time.Second
	// DefaultStallS is the hard-kill threshold — after this many seconds of
	// no stdout-log output the observer signals the runner to SIGTERM the
	// subagent. ADR-0030; configured by ObserverPolicy.StallS.
	DefaultStallS = 600 * time.Second
	// DefaultNudgeS is the soft-stall nudge threshold (cycle-124 Task 6 /
	// operator redirect): when an agent emits no fresh output for this many
	// seconds, the observer appends ONE nudge envelope to the agent's inbox
	// asking it to summarize state + continue OR finalize, BEFORE the hard
	// SIGTERM at DefaultStallS. Default is half of DefaultStallS so the
	// agent gets a clear "still alive?" prompt with enough time to recover.
	// Opt-out: set ObserverPolicy.NudgeS=0. See ADR-0023 facet A.
	DefaultNudgeS = 300 * time.Second

	// observerEventsSuffix names the observer's own NDJSON sink file (the
	// adapter writes <phase>-observer-events.ndjson into WorkspaceDir). It is
	// excluded from the workspace-activity scan so the observer's own
	// started/stall/stopped writes can never reset the stall timer and mask a
	// genuine stall. Must match the adapter's eventsPath convention
	// (core_adapter.go).
	observerEventsSuffix = "-observer-events.ndjson"

	// activityScanMaxFiles bounds the per-poll workspace walk. The per-phase
	// workspace (.evolve/runs/cycle-N) is small and the per-cycle git worktree
	// lives OUTSIDE it, so a real run never approaches this; the cap is a
	// defensive backstop against a pathological tree.
	activityScanMaxFiles = 500
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
	StallS    time.Duration // ObserverPolicy.StallS
	PollS     time.Duration // ObserverPolicy.PollS
	Cycle     int
	Phase     string
	Agent     string
	StdoutLog string // path to subagent stdout-log file
	// WorkspaceDir, when non-empty, makes a fresh write anywhere under it
	// count as progress alongside stdout-log growth. This is load-bearing for
	// tmux-driver agents (claude-tmux/codex-tmux/agy-tmux): their live output
	// goes to the tmux scrollback and reaches the stdout-log only on clean
	// exit, so the stdout-log stays flat while the agent is productively
	// writing artifacts. Without this signal the observer falsely reports
	// stall_no_output for a working tmux agent (cycle-141). Empty → stdout-log
	// growth is the only progress signal (byte-identical to the pre-fix path).
	WorkspaceDir string

	// LivenessProbe, when non-nil, is consulted ONLY at the stall threshold,
	// before a stall_no_output incident is emitted. It reports whether the
	// agent is still alive by a signal the filesystem cannot see: for
	// tmux-driver phases, whether the live tmux pane changed since the last
	// check (the spinner / token-counter advancing during a long single
	// "Incubating" turn that commits no scrollback lines and writes no
	// artifact yet). On true, the observer treats the agent as alive — it
	// resets the stall clock and emits a benign stall_probe_active info event
	// instead of a false incident (cycle-190). The probe is consulted at most
	// once per StallS window, so a subprocess-backed probe (tmux capture-pane)
	// costs nothing on the common no-stall path. Nil → no probe; stdout-log +
	// workspace growth govern alone (byte-identical to the pre-probe path).
	LivenessProbe func() bool

	// OnEvent, when non-nil, is invoked synchronously for every emission with the
	// typed Event, after the NDJSON sink write and OUTSIDE the sink lock. It makes
	// the observer's events subscribable in-process so a consumer (or a test) can
	// react to a stall/started/stopped emission without tailing the NDJSON file.
	// Implementations MUST NOT block (use a buffered or non-blocking send) — a slow
	// subscriber would otherwise stall the watch loop. Nil → no hook (byte-identical
	// to the sink-only path).
	OnEvent func(Event)
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
			// Progress = stdout-log grew OR a fresh write landed in the
			// workspace tree (the tmux-driver signal). Either resets the timer.
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
				// Before declaring a stall, consult the liveness probe (if any).
				// A tmux agent mid-"Incubating" turn produces no filesystem
				// growth but is alive (pane spinner advancing) — holding the
				// kill here avoids the cycle-190 false-positive. The probe is
				// only reached at the threshold, so it never runs on the common
				// healthy path.
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

// newestActivity returns the most recent mtime among regular files under
// cfg.WorkspaceDir, excluding the observer's own events file (see
// observerEventsSuffix). Returns the zero Time when WorkspaceDir is unset
// (disables the signal — stdout-log growth then governs alone) or empty.
// The walk is bounded by activityScanMaxFiles; walk errors degrade to "no
// fresh activity this tick" (the ticker retries) rather than failing Watch.
func (o *Observer) newestActivity() time.Time {
	if o.cfg.WorkspaceDir == "" {
		return time.Time{}
	}
	var newest time.Time
	seen := 0
	_ = filepath.Walk(o.cfg.WorkspaceDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries; keep walking
		}
		if info.IsDir() {
			return nil
		}
		// Exclude the observer's own sink BEFORE charging the cap — it is not
		// part of the work budget, and excluding it can never reset the timer.
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
	o.encMu.Lock()
	if b, err := json.Marshal(e); err == nil {
		_, _ = o.sink.Write(append(b, '\n'))
	}
	o.encMu.Unlock()
	// In-process subscribers (nil in production) react to the typed event after the
	// sink write, outside the lock so a blocking subscriber can't stall other emits.
	if o.cfg.OnEvent != nil {
		o.cfg.OnEvent(e)
	}
}
