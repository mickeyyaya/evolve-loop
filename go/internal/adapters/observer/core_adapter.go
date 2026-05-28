package observer

// core_adapter.go — cycle-122 Fix 3 / ADR-0030: bridges this package's
// per-phase Observer + Watch loop to the orchestrator's core.Observer
// interface so `evolve loop` can auto-spawn a stall detector for every
// phase without requiring the operator to run `evolve phase-observer`
// as a separate subprocess.
//
// The adapter is intentionally thin: it translates core.PhaseRequest
// → Config, derives the stdout-log path from <workspace>/<phase>-
// stdout.log (matching the runner's convention at
// go/internal/phases/runner/runner.go), opens/creates an append-only
// events sink at <workspace>/<phase>-observer-events.ndjson, and runs
// the existing Observer.Watch goroutine. The returned cancel function
// signals the watcher to stop + closes the events sink.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// CoreAdapter satisfies the core.Observer interface by spawning one
// Observer.Watch goroutine per Start call. Defaults come from the
// EVOLVE_OBSERVER_* env vars (StallS, PollS) resolved at Start time so
// per-cycle env overrides apply. Zero value is a usable adapter with
// production defaults (StallS=600s, PollS=5s).
type CoreAdapter struct {
	// Sink, when non-nil, replaces the default <workspace>/<phase>-
	// observer-events.ndjson file sink. Used by tests + future
	// reviewers that want to consume events in-process.
	Sink io.Writer

	// EnvLookup overrides os.Getenv for tests. Nil → os.Getenv.
	EnvLookup func(key string) string
}

// NewCoreAdapter returns a CoreAdapter wired with production defaults.
// Operators usually call this from cmd_cycle.go's wireOrchestratorDeps
// when EVOLVE_OBSERVER_AUTOSPAWN != "0".
func NewCoreAdapter() *CoreAdapter {
	return &CoreAdapter{}
}

// Start implements core.Observer. Returns a cancel function the
// orchestrator MUST call when the phase finishes. The cancel is
// idempotent + waits up to 2s for the watcher goroutine to exit
// (so the events sink is fully flushed before the next phase starts).
func (a *CoreAdapter) Start(ctx context.Context, phase string, req core.PhaseRequest) func() {
	if req.Workspace == "" || phase == "" {
		// No workspace path = no stdout log to watch (e.g., pre-cycle
		// orchestrator hooks). No-op cancel.
		return func() {}
	}
	stdoutLog := filepath.Join(req.Workspace, phase+"-stdout.log")
	eventsPath := filepath.Join(req.Workspace, phase+"-observer-events.ndjson")

	sink := a.Sink
	var sinkCloser io.Closer
	if sink == nil {
		// Append-only NDJSON sink. Mkdir best-effort — workspace is
		// expected to exist by phase-start (orchestrator created it).
		_ = os.MkdirAll(req.Workspace, 0o755)
		f, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			// Best-effort: print to stderr + degrade to no-op. Per
			// ADR-0030, observer failure must NOT block the phase.
			fmt.Fprintf(os.Stderr, "[observer] WARN open events file %s: %v (degraded: no auto-spawn this phase)\n", eventsPath, err)
			return func() {}
		}
		sink = f
		sinkCloser = f
	}

	cfg := Config{
		StallS:    a.resolveDuration("EVOLVE_OBSERVER_STALL_S", DefaultStallS),
		PollS:     a.resolveDuration("EVOLVE_OBSERVER_POLL_S", DefaultPollS),
		Cycle:     req.Cycle,
		Phase:     phase,
		Agent:     phase, // runner-side: agent name == phase name post-prefix-strip
		StdoutLog: stdoutLog,
	}
	obs := New(cfg, sink)

	watchCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = obs.Watch(watchCtx) // ctx.Err() or nil on Stop — neither is fatal here
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			_ = obs.Stop()
			// Bounded wait so a deadlocked watcher can't hold up the
			// orchestrator. The watcher's ticker loop checks ctx.Done
			// every PollS (5s default) so 2s is sometimes too short;
			// raise to 10s to cover the worst legitimate case.
			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				fmt.Fprintf(os.Stderr, "[observer] WARN watcher for phase=%s didn't exit within 10s; leaking goroutine (cycle should still complete)\n", phase)
			}
			if sinkCloser != nil {
				_ = sinkCloser.Close()
			}
		})
	}
}

// resolveDuration reads an EVOLVE_OBSERVER_* env var as seconds; falls
// back to def on empty/invalid. Honors the per-adapter EnvLookup test
// seam.
func (a *CoreAdapter) resolveDuration(key string, def time.Duration) time.Duration {
	get := os.Getenv
	if a.EnvLookup != nil {
		get = a.EnvLookup
	}
	raw := get(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	return time.Duration(n) * time.Second
}
