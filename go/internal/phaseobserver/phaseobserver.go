// Package phaseobserver hosts the standalone `evolve phase-observer` stall detector, driving internal/observerengine.
// See docs/architecture/packages/internal-phaseobserver.md.
package phaseobserver

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/inbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/observerengine"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Exit codes Run returns.
const (
	ExitOK          = 0
	ExitInvalidArgs = 10
)

// Scope is "phase" (the default) or "cycle"; any other value runs no stall rule.
type Scope string

// Scope values, spelled as the engine spells them.
const (
	ScopePhase Scope = observerengine.ScopePhase
	ScopeCycle Scope = observerengine.ScopeCycle
)

// Config wires the observer.
// CycleState and LoopN through ThrottleN are read by nothing; they keep the literal shape callers build.
type Config struct {
	Workspace    string
	SubagentPGID int
	Cycle        int
	Phase        string
	Agent        string
	CycleState   string
	Scope        Scope
	Enforce      bool

	PollS          int
	StallS         int
	NudgeS         int    // soft-stall nudge threshold; 0 disables the nudge
	NudgeBody      string // text appended to the agent inbox on soft stall
	LoopN          int
	LoopWindowS    int
	ErrorRate      float64
	CostSigma      float64
	ThrottleN      int
	EOFGraceS      int
	HeartbeatEvery int

	// MaxNoProgressS fires stuck_no_progress when no tool_use or tool_result arrives for this long;
	// StallS resets on any line, so it misses a babbling agent. 0 disables it.
	MaxNoProgressS int

	// StallPolicy decides each stall INCIDENT's action and records it in the envelope; its
	// verdict outranks Enforce. nil keeps the Enforce kill and an unenriched envelope.
	StallPolicy recovery.StallPolicy

	// ProcessAlive probes the agent's process group; a dead group fires one process_dead
	// INCIDENT. nil disables the probe, so fixture Configs with fake pgids never read as dead.
	ProcessAlive func(pgid int) bool

	// Signals is the late-bound Signal Center accessor for the engine's own faults; nil is silent.
	Signals func() *signalcenter.Center

	Now         func() time.Time
	KillPgrp    func(pgid int, sig syscall.Signal) error
	ShutdownSig <-chan struct{} // SIGUSR1-equivalent
	StopAfterMS int             // testing: force shutdown after this many ms
}

// DefaultProcessAlive is the production liveness probe: signal 0 to the process group.
// EPERM also reads as dead, which is safe only while observer and agent share a UID.
func DefaultProcessAlive(pgid int) bool {
	return syscall.Kill(-pgid, 0) == nil
}

// stopChan returns nil for a nil timer: a nil channel's select arm never fires and allocates nothing.
func stopChan(stopTimer *time.Timer) <-chan time.Time {
	if stopTimer != nil {
		return stopTimer.C
	}
	return nil
}

// Run drives the observer until shutdown and returns ExitOK or ExitInvalidArgs.
// The engine it builds is never shared: this goroutine is its only caller, so it needs no locking.
func Run(cfg Config, stdoutPath string, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[phase-observer] "+format+"\n", args...)
	}

	if cfg.Workspace == "" || cfg.Phase == "" || cfg.Agent == "" {
		logf("usage: phase-observer <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]")
		return ExitInvalidArgs
	}
	if info, err := os.Stat(cfg.Workspace); err != nil || !info.IsDir() {
		logf("workspace not a directory: %s", cfg.Workspace)
		return ExitInvalidArgs
	}
	if cfg.Cycle <= 0 {
		logf("cycle must be integer")
		return ExitInvalidArgs
	}

	cfg = withDefaults(cfg)
	paths := observerengine.PathsFor(cfg.Workspace, cfg.Agent)
	if stdoutPath != "" {
		paths.Stdout = stdoutPath
	}
	eng := wiredEngine(cfg, paths)
	eng.Start()
	pollLoop(cfg, eng)
	// The engine already reports a failed write as OBSERVER_REPORT_WRITE_FAILED; the report never changes the exit code.
	_ = eng.WriteReport()
	return ExitOK
}

// withDefaults applies the host's defaults; PollS and StallS must equal policy's compiled ObserverConfig.
// ProcessAlive gets no default: nil keeps the probe off.
func withDefaults(cfg Config) Config {
	if cfg.PollS == 0 {
		cfg.PollS = 5
	}
	if cfg.StallS == 0 {
		cfg.StallS = 600
	}
	if cfg.NudgeS > 0 && cfg.NudgeBody == "" {
		cfg.NudgeBody = "You appear stalled. Summarize your current state, then either continue or finalize your artifact."
	}
	if cfg.EOFGraceS == 0 {
		cfg.EOFGraceS = 10
	}
	if cfg.HeartbeatEvery == 0 {
		cfg.HeartbeatEvery = 12
	}
	if cfg.Scope == "" {
		cfg.Scope = ScopePhase
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.KillPgrp == nil {
		cfg.KillPgrp = func(pgid int, sig syscall.Signal) error {
			return syscall.Kill(-pgid, sig)
		}
	}
	return cfg
}

// wiredEngine is the module's only construction of the engine; its nudge port appends to the agent's bridge inbox.
func wiredEngine(cfg Config, paths observerengine.Paths) *observerengine.Engine {
	return observerengine.New(settingsOf(cfg, paths), observerengine.Deps{
		Now:          cfg.Now,
		ProcessAlive: cfg.ProcessAlive,
		Kill:         cfg.KillPgrp,
		Nudge: func(body string) error {
			return inbox.Append(cfg.Workspace, cfg.Agent, inbox.Envelope{
				Kind:   inbox.KindNudge,
				Body:   body,
				Source: "observer",
			}, cfg.Now)
		},
		Policy: cfg.StallPolicy,
	}, observerengine.WithSignals(cfg.Signals))
}

// settingsOf projects Config onto the engine's Settings and binds the observer's own pid.
func settingsOf(cfg Config, paths observerengine.Paths) observerengine.Settings {
	return observerengine.Settings{
		Cycle: cfg.Cycle, Phase: cfg.Phase, Agent: cfg.Agent, Scope: string(cfg.Scope),
		Enforce: cfg.Enforce, PGID: cfg.SubagentPGID, PID: os.Getpid(),
		PollS: cfg.PollS, StallS: cfg.StallS, NudgeS: cfg.NudgeS, EOFGraceS: cfg.EOFGraceS,
		HeartbeatEvery: cfg.HeartbeatEvery, MaxNoProgressS: cfg.MaxNoProgressS,
		NudgeBody: cfg.NudgeBody, Paths: paths,
	}
}

// pollLoop owns the real ticker, the optional stop timer and the shutdown signal; each arm names its reason.
func pollLoop(cfg Config, eng *observerengine.Engine) {
	tickerInterval := time.Duration(cfg.PollS) * time.Second
	if cfg.StopAfterMS > 0 {
		tickerInterval = time.Duration(cfg.StopAfterMS) * time.Millisecond / 4
	}
	pollTicker := time.NewTicker(tickerInterval)
	defer pollTicker.Stop()

	var stopTimer *time.Timer
	if cfg.StopAfterMS > 0 {
		stopTimer = time.NewTimer(time.Duration(cfg.StopAfterMS) * time.Millisecond)
	}
	stopCh := stopChan(stopTimer)

	for {
		select {
		case <-cfg.ShutdownSig:
			eng.Shutdown("sigusr1")
			return
		case <-stopCh:
			eng.Shutdown("stop-timer")
			return
		case <-pollTicker.C:
			if eng.Tick() == observerengine.StopEOFGrace {
				eng.Shutdown("eof_grace")
				return
			}
		}
	}
}
