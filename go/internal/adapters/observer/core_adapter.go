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
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/channel"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// CoreAdapter satisfies the core.Observer interface by spawning one
// Observer.Watch goroutine per Start call. Zero value is a usable adapter
// with production defaults (StallS=600s, PollS=5s).
type CoreAdapter struct {
	// Sink, when non-nil, replaces the default <workspace>/<phase>-
	// observer-events.ndjson file sink. Used by tests + future
	// reviewers that want to consume events in-process.
	Sink io.Writer

	// EnvLookup overrides os.Getenv for tests. Nil → os.Getenv.
	EnvLookup func(key string) string

	// RecoveryStage is the ADR-0044 Unified Phase Recovery stage, injected
	// by the orchestrator from cfg.PhaseRecovery (policy-resolved). Empty →
	// channel.ResolveStage returns "shadow" (behavior-neutral default).
	RecoveryStage string

	// Config carries policy.json observer settings.
	Config policy.ObserverPolicy
}

// NewCoreAdapter returns a CoreAdapter wired with production defaults.
func NewCoreAdapter(config ...policy.ObserverPolicy) *CoreAdapter {
	a := &CoreAdapter{}
	if len(config) > 0 {
		a.Config = config[0]
	}
	return a
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

	resolved := policy.Policy{Observer: &a.Config}.ObserverConfig()
	cfg := Config{
		StallS:    time.Duration(*resolved.StallS) * time.Second,
		PollS:     time.Duration(*resolved.PollS) * time.Second,
		Cycle:     req.Cycle,
		Phase:     phase,
		Agent:     phase, // runner-side: agent name == phase name post-prefix-strip
		StdoutLog: stdoutLog,
		// Treat fresh writes anywhere in the workspace as progress — tmux-driver
		// agents write live output to the tmux scrollback (not the stdout-log),
		// so workspace artifact writes are the only filesystem liveness signal
		// until clean exit. Without this the observer falsely stalls a working
		// tmux build agent (cycle-141).
		WorkspaceDir: req.Workspace,
		// cycle-190 + headless follow-up: when even workspace writes go quiet
		// (a long single "Incubating" turn that thinks for minutes then dumps
		// its artifact), filesystem signals are blind. Two ground-truth probes,
		// consulted ONLY at the stall threshold and OR-ed (alive if either):
		//   - tmux pane-hash: the live pane spinner/token-counter advancing
		//     (tmux-driver phases; a non-tmux phase finds no session → no-op);
		//   - process CPU time: the agent subprocess accruing CPU (HEADLESS
		//     phases, which have no pane; PID written by the bridge at launch).
		LivenessProbe: anyProbe(
			newTmuxPaneProbe(req.Cycle, phase, req.RunID, nil),
			newProcessCPUProbe(core.BridgePIDFile(stdoutLog), nil),
		),
	}
	// Cycle-124 Task 6 — KNOWN GAP: the operator's "active liveness
	// nudging" mechanism is wired into the STANDALONE `evolve phase-
	// observer` (cmd_phase_observer.go default flipped 0 → 300s) but the
	// AUTO-SPAWN path here does NOT yet emit nudge envelopes — this
	// adapter's Observer is a thin Watch-only implementation; the full
	// nudge logic (inbox append + nudged dedupe + soft_stall_nudge event)
	// lives in `internal/phaseobserver` and would need porting. The
	// resolveNudgeS + DefaultNudgeS scaffolding in this package is ready
	// for that follow-up. Tracked as the cycle-124 backlog item: "auto-
	// spawn observer nudge wire-up". For autonomous `evolve loop` runs,
	// nudging is currently effectively opt-out (no nudge fires unless an
	// operator runs the standalone phase-observer alongside the loop).
	obs := New(cfg, sink)

	watchCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = obs.Watch(watchCtx) // ctx.Err() or nil on Stop — neither is fatal here
	}()

	// ADR-0037 + ADR-0045 I6: spawn the live channel producer beside the
	// observer when the channel is on. The channel rides EVOLVE_PHASE_RECOVERY
	// (enforce implies it). channel.Enabled is the single source shared with the
	// bridge driver. Off → byte-identical to pre-channel behavior (no producer,
	// no feed file).
	var prodCancel func()
	chOn := channel.Enabled(channel.ResolveStage(a.RecoveryStage))
	if chOn {
		// Transport-aware source (ADR-0037 RT3): a tmux-family driver streams its
		// live answer to <agent>-pane.live and breadcrumbs to
		// <agent>-breadcrumbs.live (its stdout.log is empty until the at-exit
		// dump), so point the Producer there. Headless → empty paths → the
		// Producer keeps its legacy <phase>-stdout/-stderr.log defaults.
		stdoutPath, stderrPath := a.channelSourcePaths(req, phase)
		p := channel.NewProducer(channel.ProducerConfig{
			Workspace: req.Workspace, Agent: phase, Phase: phase, Cycle: req.Cycle,
			StdoutPath: stdoutPath, StderrPath: stderrPath,
		})
		pctx, pcancel := context.WithCancel(ctx)
		prodCancel = pcancel
		wg.Add(1)
		go func() { defer wg.Done(); _ = p.Run(pctx) }()
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			if prodCancel != nil {
				prodCancel()
			}
			_ = obs.Stop()
			// Bounded wait so a deadlocked watcher can't hold up the
			// orchestrator. The watcher's ticker loop checks ctx.Done
			// every PollS (5s default) so 2s is sometimes too short;
			// raise to 10s to cover the worst legitimate case.
			done := make(chan struct{})
			go func() { wg.Wait(); close(done) }()
			if !closeSinkAfterWait(done, 10*time.Second, sinkCloser) {
				fmt.Fprintf(os.Stderr, "[observer] WARN watcher for phase=%s didn't exit within 10s; leaking goroutine and leaving its events-sink fd open on purpose — closing it would race the watcher's writes; the OS reclaims it at process exit (cycle should still complete)\n", phase)
			}
		})
	}
}

// closeSinkAfterWait waits up to timeout for done to close, then closes closer
// ONLY if done fired within the window — never on the timeout arm, so a
// still-running leaked watcher goroutine's sink is never closed out from under
// it (the use-after-close race the 10s bound describes). A nil closer is a
// no-op, preserving Start's `if sinkCloser != nil` contract. Returns true iff
// done fired within timeout (false → the caller may WARN about the leak).
func closeSinkAfterWait(done <-chan struct{}, timeout time.Duration, closer io.Closer) bool {
	select {
	case <-done:
		if closer != nil {
			_ = closer.Close()
		}
		return true
	case <-time.After(timeout):
		return false
	}
}

// channelSourcePaths returns the (stdout, stderr) files the channel Producer
// should tail for this phase (ADR-0037 RT3). A tmux-family driver streams its
// live answer to <agent>-pane.live + correlation breadcrumbs to
// <agent>-breadcrumbs.live, so the producer reads that pair. A headless driver
// streams live to <phase>-stdout.log, so empty strings are returned and the
// producer keeps its legacy defaults. The family is resolved best-effort from
// the per-phase CLI env (profile.cli pins not surfaced in env are not seen
// here — a wrong guess only degrades that phase's live feed, never the phase).
func (a *CoreAdapter) channelSourcePaths(req core.PhaseRequest, phase string) (stdout, stderr string) {
	if !bridge.IsTmuxDriver(a.phaseCLI(req, phase)) {
		return "", ""
	}
	return filepath.Join(req.Workspace, phase+"-pane.live"),
		filepath.Join(req.Workspace, phase+"-breadcrumbs.live")
}

// phaseCLI resolves the per-phase CLI/driver name from the per-cycle request
// snapshot and defaults to llmroute's "claude-tmux". Persistent per-agent
// selection belongs to the profile SSOT; this best-effort observer does not
// load profiles. The agent key uppercases the phase with hyphens → underscores
// (tdd-engineer → EVOLVE_TDD_ENGINEER_CLI).
func (a *CoreAdapter) phaseCLI(req core.PhaseRequest, phase string) string {
	look := func(k string) string {
		if v, ok := req.Env[k]; ok && v != "" {
			return v
		}
		return ""
	}
	agentKey := "EVOLVE_" + strings.ToUpper(strings.ReplaceAll(phase, "-", "_")) + "_CLI"
	if v := look(agentKey); v != "" {
		return v
	}
	if v := look("EVOLVE_CLI"); v != "" {
		return v
	}
	return "claude-tmux"
}

// resolveDuration reads an injected value as seconds and falls back to def
// on empty/invalid OR on a non-positive value. Used for
// PollS/StallS where 0 is meaningless (no polling / no stall threshold)
// and indistinguishable from "the operator forgot to set it". Per-adapter
// EnvLookup is honored. For NudgeS the 0-means-disable semantics differ —
// see resolveNudgeS below.
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

// resolveString reads an injected string setting and falls back on empty.
func (a *CoreAdapter) resolveString(key, def string) string {
	get := os.Getenv
	if a.EnvLookup != nil {
		get = a.EnvLookup
	}
	if raw := get(key); raw != "" {
		return raw
	}
	return def
}

// resolveNudgeS resolves an injected seconds setting where zero explicitly
// disables nudging and invalid/negative values fall back to the default.
func (a *CoreAdapter) resolveNudgeS(key string, def time.Duration) time.Duration {
	get := os.Getenv
	if a.EnvLookup != nil {
		get = a.EnvLookup
	}
	raw := get(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return def
	}
	return time.Duration(n) * time.Second
}
