// `evolve loop` drives the cycle dispatcher loop with batch budget
// enforcement. Sequential by design: each cycle blocks the next until
// it completes or trips the batch cap.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	// Blank import: checkpoint's init() registers the orchestrator's
	// phase-boundary checkpointer; without it checkpointing silently no-ops.
	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

var validStrategies = map[string]struct{}{
	"balanced":     {},
	"innovate":     {},
	"harden":       {},
	"repair":       {},
	"ultrathink":   {},
	"autoresearch": {},
}

type loopConfig struct {
	ProjectRoot string `json:"project_root"`
	EvolveDir   string `json:"evolve_dir"`
	GoalHash    string `json:"goal_hash"`
	GoalText    string `json:"goal_text,omitempty"`
	Strategy    string `json:"strategy"`
	MaxCycles   int    `json:"max_cycles"`
	// MaxCyclesExplicit is true only when the operator set --max-cycles/--cycles
	// or a positional count; false lets an enforced cycle-budget policy raise
	// the ceiling to the safety cap instead of defaulting to 1.
	MaxCyclesExplicit bool `json:"max_cycles_explicit,omitempty"`
	Resume            bool `json:"resume,omitempty"`
	Reset             bool `json:"reset,omitempty"`
	// Fingerprint is the --fingerprint value for --reset: it acknowledges one
	// failure fingerprint so the blocker-breaker excludes it going forward.
	// See ADR-0072.
	Fingerprint       string            `json:"fingerprint,omitempty"`
	ConsensusAudit    bool              `json:"consensus_audit,omitempty"`
	DryRun            bool              `json:"dry_run,omitempty"`
	ForceFresh        bool              `json:"force_fresh,omitempty"`
	SkipPreflight     bool              `json:"skip_preflight,omitempty"`
	SkipPreflightBoot bool              `json:"skip_preflight_boot,omitempty"`
	BypassPolicy      bool              `json:"bypass_policy,omitempty"`
	ChainMode         bool              `json:"chain_mode,omitempty"`
	PerAgentCLI       map[string]string `json:"per_agent_cli,omitempty"`
	PerAgentModel     map[string]string `json:"per_agent_model,omitempty"`
	PreflightOnly     bool              `json:"preflight_only,omitempty"`
	Detach            bool              `json:"detach,omitempty"`
	LogPath           string            `json:"log_path,omitempty"`
	DetachArgv        []string          `json:"-"`
	ResumeWaves       int               `json:"-"`
	HandedOff         bool              `json:"-"`
}

// emitSignalStop assumes the caller polled ctx.Err() before the cycle error,
// so a cancellation reads as a clean signal stop, not "context canceled".
func emitSignalStop(stdout, stderr io.Writer, lr *loopResult, cycle int) {
	signalStop(stdout, stderr, lr, fmt.Sprintf("at cycle %d — checkpointed", cycle))
}

// signalStop is the one interrupt disposition; every caller shares it so the
// stop reason, resume hint and result emit never fork.
func signalStop(stdout, stderr io.Writer, lr *loopResult, where string) {
	fmt.Fprintf(stderr, "[loop] received interrupt (SIGINT/SIGTERM) %s; resume with: evolve loop --resume\n", where)
	lr.StopReason = "signal"
	lr.emit(stdout)
}

// runLoop is the `evolve loop` entry point. Chaining is never entered
// implicitly: an absent flag and an absent policy block both leave it off.
func runLoop(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if isLoopStatusInvocation(args) {
		return runLoopStatus(args[1:], stdout, stderr)
	}
	cfg, rc := parseLoopArgs(args, stderr)
	if rc != 0 {
		return rc
	}
	if cfg.PreflightOnly {
		return runLoopPreflightOnly(cfg, stdout, stderr)
	}
	if cfg.Detach {
		return runLoopDetached(cfg, stdout, stderr)
	}
	cfg.ResumeWaves, cfg.HandedOff = takeReexecHandoff(cfg.EvolveDir, stderr)
	chainCfg := loadChainConfig(cfg.EvolveDir)
	cfg.ChainMode = cfg.ChainMode || chainCfg.Enabled

	if cfg.ChainMode && cfg.Resume {
		fmt.Fprintln(stderr, "[chain] --resume runs a single batch; chaining is not applied to a resumed cycle — relaunch with --until-inbox-empty once it completes")
	}
	if cfg.ChainMode && !cfg.DryRun && !cfg.Resume {
		return runLoopChain(cfg, chainCfg, stdin, stdout, stderr)
	}
	return runLoopBatch(cfg, stdin, stdout, stderr)
}

// runLoopBatch runs exactly one batch of cycles.
func runLoopBatch(cfg loopConfig, _ io.Reader, stdout, stderr io.Writer) int {
	maybePrintSetupNudge(stderr, cfg.EvolveDir)

	// A loop launched into the primary checkout shares its tree with the
	// operator console; classification failure is non-fatal, the tripwire
	// just stays dark.
	// See ADR-0080.
	if pi, perr := plane.Classify(cfg.ProjectRoot); perr == nil {
		fmt.Fprintln(stderr, plane.BootLine(pi))
	} else {
		fmt.Fprintf(stderr, "[loop] WARN: plane classification failed (%v) — the ADR-0080 primary-checkout tripwire is dark this run\n", perr)
	}

	if cfg.DryRun {
		buf, _ := json.MarshalIndent(map[string]any{
			"dry_run": true,
			"config":  cfg,
		}, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		return 0
	}

	runtime := startLoopBatchRuntime()
	defer runtime.close()
	ctx := runtime.ctx

	// Reaps tmux sessions left by a prior crashed run, before any cycle runs:
	// a SIGKILL'd loop never ran its teardown, so the per-run registry reaper
	// cannot see them. Liveness-scoped, so a live concurrent run is never touched.
	gcOrphanSessions("startup", stderr)

	deps := wireOrchestratorDepsFn(cfg.ProjectRoot, cfg.EvolveDir, stderr)
	defer deps.Signals.Flush()
	// orch is narrowed to loopCycleRunner so a test can inject a scripted
	// orchestrator (loopOrchOverride); the real *core.Orchestrator cannot be
	// driven to emit FinalVerdict=FAIL without a faithful phase machine. A nil
	// override runs the production orchestrator unchanged.
	var orch loopCycleRunner = deps.Orchestrator
	if loopOrchOverride != nil {
		orch = loopOrchOverride
	}
	wc := loadWorkflowConfig(cfg.EvolveDir)
	cycleclassify.SetHangClassifier(loadClassifyConfig(cfg.EvolveDir).HangClassifier)

	maintainBatchState(cfg, wc.AutoPrune, stderr)

	cycleEnv := buildCycleEnv(cfg, os.Environ())
	cycleCtx := buildCycleContext(cfg)

	lr := loopResult{StopReason: "max_cycles", classifyRoot: cfg.ProjectRoot}
	if last, lerr := readLastCycleNumber(ctx, deps.Storage); lerr == nil {
		lr.batchFirstCycle = last + 1
	}

	if cfg.Resume {
		return runResumeBatch(ctx, cfg, orch, deps.Ledger, deps.Signals, cycleEnv, cycleCtx, &lr, stdout, stderr)
	}

	exitCode, halt := prepareFreshBatch(ctx, cfg, deps, &lr, stdout, stderr)
	if halt {
		return exitCode
	}

	return (&loopBatchCoordinator{
		ctx:          ctx,
		cfg:          cfg,
		deps:         deps,
		orch:         orch,
		cycleEnv:     cycleEnv,
		cycleContext: cycleCtx,
		result:       &lr,
		workflow:     wc,
		stdout:       stdout,
		stderr:       stderr,
	}).run()
}

func isTaskLevelFailure(c cycleclassify.Classification) bool {
	return cycleoutcome.IsTaskLevelFailure(c)
}

func readCarryoverCount(statePath string) (count int, ok bool) {
	b, err := os.ReadFile(statePath)
	if err != nil {
		return 0, false
	}
	var s struct {
		CarryoverTodos []json.RawMessage `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return 0, false
	}
	return len(s.CarryoverTodos), true
}

// finalizeCompletedCycle clears a completed cycle's on-disk cycle-state.json
// marker at a clean batch exit, so the operator need not run
// `evolve cycle reset --force` before the next launch.
func finalizeCompletedCycle(cfg loopConfig, stderr io.Writer) {
	cleared, err := core.ClearCompletedCycleMarker(cfg.EvolveDir, core.FinalizeOptions{Now: time.Now, PidAlive: pidAlive})
	if err != nil {
		fmt.Fprintf(stderr, "[loop] finalize: %v\n", err)
		return
	}
	if cleared {
		fmt.Fprintf(stderr, "[loop] cleared completed cycle-state.json marker (clean exit) — no `evolve cycle reset` needed before the next launch\n")
	}
}
