// Package phaseobserver ports the core behavior of
// legacy/scripts/dispatch/phase-observer.sh (v10.18.0+ default stall detector).
//
// SCOPE NOTE: this port covers the primary use case — stall detection +
// NDJSON event tracking + report writing on shutdown. The 4 secondary
// detection rules (infinite_loop, error_spike, cost_anomaly, rate_limit) were
// never ported; their Config fields stay for the literal shape (ADR-0103
// unit 12, operator question 5).
//
// Since ADR-0103 unit 12 this file is the SEAM: validation, defaults, the
// real ticker and the select loop stay here and drive the engine in
// internal/observerengine one Tick at a time — the tail, the decoder, the
// stall rules, the incident responder and the envelope/report sinks live
// there. Run's spelling is kept for the phasecmd subcommand and the by-name
// tests (Strangler Fig). The observer emits to:
//   - {agent}-observer-events.ndjson  (live, append-only, one envelope/line)
//   - {agent}-observer-report.json    (atomic write at shutdown)
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

const (
	ExitOK          = 0
	ExitInvalidArgs = 10
)

// Scope is "phase" (default) or "cycle"; any other value runs no stall rule
// (the engine's preserved check). The spellings are the engine's.
type Scope string

const (
	ScopePhase Scope = observerengine.ScopePhase
	ScopeCycle Scope = observerengine.ScopeCycle
)

// Config wires the observer.
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
	NudgeS         int    // soft-stall nudge threshold; 0 = disabled (opt-in)
	NudgeBody      string // text appended to the agent inbox on soft stall
	LoopN          int
	LoopWindowS    int
	ErrorRate      float64
	CostSigma      float64
	ThrottleN      int
	EOFGraceS      int
	HeartbeatEvery int

	// MaxNoProgressS is Workstream E1's "babbling agent" backstop. The existing
	// StallS check resets on ANY valid JSON line — including pure assistant_text
	// token streaming — so a livelocked agent that emits forever never trips it.
	// MaxNoProgressS tracks "last MEANINGFUL progress" (tool_use / tool_result /
	// progress events only) and emits stuck_no_progress when exceeded. 0 = the
	// feature is disabled (legacy posture, byte-identical to pre-E1).
	MaxNoProgressS int

	// StallPolicy (ADR-0044 C4) maps a stall INCIDENT to a recovery action
	// (extend | kill_retry | escalate), decoupling action from detection.
	// nil — the default until the C3 composition slice wires a real policy —
	// preserves the legacy inline behavior byte-for-byte: Enforce → SIGTERM,
	// unenriched INCIDENT envelope. With a policy injected, its verdict
	// outranks Enforce (extend/escalate suppress the kill; kill_retry kills
	// even without Enforce) and the decision + justification are recorded
	// inside the INCIDENT envelope (action / action_reason).
	StallPolicy recovery.StallPolicy

	// ProcessAlive probes whether the observed agent's process group still
	// exists (R3.4, cycles 274/277: pane/log echo is NOT liveness — a wedged
	// shell read as alive for 25+ min after the CLI died). A dead group
	// fires a "process_dead" INCIDENT once, resolved by the stall policy
	// (chain: kill_retry) regardless of idle budgets. nil DISABLES the probe
	// (legacy byte-identical — the same nil-seam convention as StallPolicy);
	// the composition root wires DefaultProcessAlive, so fixture Configs
	// with fake pgids never see a false "dead".
	ProcessAlive func(pgid int) bool

	// Signals is the Signal Center accessor the engine reports its own faults
	// through (observer.warning under module observer; late-bound; nil = the
	// Null Object). Wired by the composition root (phasecmd), never here.
	Signals func() *signalcenter.Center

	// Testing seams.
	Now         func() time.Time
	KillPgrp    func(pgid int, sig syscall.Signal) error
	ShutdownSig <-chan struct{} // SIGUSR1-equivalent
	StopAfterMS int             // testing: force shutdown after this many ms
}

// DefaultProcessAlive is the production R3.4 liveness probe: signal-0 to the
// process group — ESRCH (an error) means the group is gone. EPERM (also an
// error) is treated as DEAD here, which is conservative-but-safe only
// because observer and agent share a UID; if cross-UID subagents are ever
// introduced this probe must distinguish ESRCH from EPERM. Wired by the
// composition root (phasecmd); Run treats nil as probe-off so fixture
// Configs with fake pgids never see a false "dead".
func DefaultProcessAlive(pgid int) bool {
	return syscall.Kill(-pgid, 0) == nil
}

// stopChan resolves the poll loop's stop-timer arm. A nil timer yields a nil
// channel — the never-fires select idiom — instead of allocating a fresh
// throwaway channel every iteration (the prior inline-func form). Hoisted once
// before the loop, so it is alloc-free per tick.
func stopChan(stopTimer *time.Timer) <-chan time.Time {
	if stopTimer != nil {
		return stopTimer.C
	}
	return nil
}

// Run drives the observer until shutdown. Returns the bash-compatible exit code.
//
// CONCURRENCY: Run constructs a local engine and drives it from THIS single
// goroutine — the poll loop is the sole caller of the engine's methods. The
// engine is never returned or shared, so its state needs no locking.
func Run(cfg Config, stdoutPath string, stderr io.Writer) int {
	logf := func(format string, args ...any) {
		fmt.Fprintf(stderr, "[phase-observer] "+format+"\n", args...)
	}

	// validation
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
	// The report is best-effort: a failed write is already reported by the
	// engine as OBSERVER_REPORT_WRITE_FAILED and never changes the exit code.
	_ = eng.WriteReport()
	return ExitOK
}

// withDefaults applies the host's defaults — the one belief owner beside
// policy's compiled ObserverConfig (TestWithDefaults_MatchPolicyCompiledDefaults).
// ProcessAlive deliberately has NO default: nil = probe off (legacy
// byte-identical); DefaultProcessAlive is wired at the composition root.
func withDefaults(cfg Config) Config {
	if cfg.PollS == 0 {
		cfg.PollS = 5
	}
	if cfg.StallS == 0 {
		cfg.StallS = 600
	}
	// NudgeS is opt-in: 0 leaves the soft-stall nudge disabled. NudgeBody
	// gets a default only when nudging is enabled.
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

// wiredEngine is the ONE construction of the engine
// (TestObserverEngine_OneConstructionSite): the defaulted Config projected
// once onto Settings and Deps, the nudge port over the bridge inbox (the
// inbox and its delivery key — the agent positional — are the host's
// concern; inbox.Append mints the envelope TS from the injected clock), the
// Center through the late-bound accessor.
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

// settingsOf is the ONE projection from Config onto the engine's Settings;
// the observer's own pid is bound here.
func settingsOf(cfg Config, paths observerengine.Paths) observerengine.Settings {
	return observerengine.Settings{
		Cycle: cfg.Cycle, Phase: cfg.Phase, Agent: cfg.Agent, Scope: string(cfg.Scope),
		Enforce: cfg.Enforce, PGID: cfg.SubagentPGID, PID: os.Getpid(),
		PollS: cfg.PollS, StallS: cfg.StallS, NudgeS: cfg.NudgeS, EOFGraceS: cfg.EOFGraceS,
		HeartbeatEvery: cfg.HeartbeatEvery, MaxNoProgressS: cfg.MaxNoProgressS,
		NudgeBody: cfg.NudgeBody, Paths: paths,
	}
}

// pollLoop is the process concern the host keeps: the real ticker, the
// optional stop timer and the shutdown signal, each arm naming its reason.
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
