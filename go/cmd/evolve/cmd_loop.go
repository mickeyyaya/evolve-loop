// `evolve loop` drives the cycle dispatcher loop with batch budget
// enforcement. Sequential by design — each cycle blocks the next until
// it completes or trips the batch cap (matches v8.34.0+ bash
// dispatcher behavior).
//
// v11.5.0 M1–M6: CLI surface mirrors the now-removed bash dispatcher —
// positional args ([CYCLES] [STRATEGY] [GOAL...]), --goal-text (computes
// hash via goalhash.Compute), --strategy, --resume, --dry-run, --reset,
// --consensus-audit. Existing --goal-hash callers continue to work
// unchanged.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	// Blank import: checkpoint's init() registers core.PhaseBoundaryCheckpointer
	// so the orchestrator writes a resumable checkpoint at every phase boundary.
	// Without this the hook stays nil and the feature silently no-ops in production.
	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

// validStrategies mirrors the bash whitelist at
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:294-298.
var validStrategies = map[string]struct{}{
	"balanced":     {},
	"innovate":     {},
	"harden":       {},
	"repair":       {},
	"ultrathink":   {},
	"autoresearch": {},
}

// loopResult is the per-invocation dispatcher output. Marshaled to
// stdout as pretty JSON at every exit point — tests grep its
// stop_reason field to assert the loop took the right path.
//
// Lifted to file scope (was previously a local type inside runLoop)
// so the .emit() helper can be a method, killing the 11 repeated
// `buf, _ := json.MarshalIndent(lr, "", "  "); fmt.Fprintln(...)`
// pattern that obscured the loop body.
type loopConfig struct {
	ProjectRoot string `json:"project_root"`
	EvolveDir   string `json:"evolve_dir"`
	GoalHash    string `json:"goal_hash"`
	GoalText    string `json:"goal_text,omitempty"`
	Strategy    string `json:"strategy"`
	MaxCycles   int    `json:"max_cycles"`
	// MaxCyclesExplicit records whether the operator set --max-cycles/--cycles
	// (or a positional count). When false and cycle-budget policy is enforce, the
	// loop defaults its ceiling to the safety cap and lets completion drive the
	// stop, instead of the legacy default of 1.
	MaxCyclesExplicit bool `json:"max_cycles_explicit,omitempty"`
	Resume            bool `json:"resume,omitempty"`
	Reset             bool `json:"reset,omitempty"`
	// Fingerprint is the operator-driven --fingerprint <fp> arg: with --reset,
	// acknowledges this exact failure fingerprint in
	// .evolve/resolved-fingerprints.json so the blocker-breaker's Rule B
	// excludes it going forward (ADR-0072 extension operator unblock — the
	// cycle-1329 recurrence fix). Empty = no-op, byte-identical pre-existing
	// --reset behavior.
	Fingerprint       string `json:"fingerprint,omitempty"`
	ConsensusAudit    bool   `json:"consensus_audit,omitempty"`
	DryRun            bool   `json:"dry_run,omitempty"`
	ForceFresh        bool   `json:"force_fresh,omitempty"`
	SkipPreflight     bool   `json:"skip_preflight,omitempty"`
	SkipPreflightBoot bool   `json:"skip_preflight_boot,omitempty"`
	BypassPolicy      bool   `json:"bypass_policy,omitempty"`
	// ChainMode is the resolved batch-chaining opt-in: the --until-inbox-empty
	// CLI parameter ORed with policy.json chain.enabled. When set, runLoop
	// drives runLoopBatch repeatedly at the batch boundary (cmd_loop_chain.go)
	// instead of returning after one batch and waiting for an operator relaunch.
	ChainMode bool `json:"chain_mode,omitempty"`
	// PerAgentCLI / PerAgentModel are the parsed `--cli` / `--model`
	// repeatable launch flags (Workstream G2). Each entry maps a profile
	// agent name (e.g. "auditor", "tdd-engineer") to the CLI / model that
	// should override the profile default for THIS loop invocation only.
	// Translated into EVOLVE_<AGENT>_CLI / EVOLVE_<AGENT>_MODEL env entries
	// inside buildCycleEnv; the runner picks them up via envchain.
	// Empty map = byte-identical pre-G2 behavior.
	PerAgentCLI   map[string]string `json:"per_agent_cli,omitempty"`
	PerAgentModel map[string]string `json:"per_agent_model,omitempty"`
}

// emitSignalStop logs a graceful, resumable stop after a SIGINT/SIGTERM
// cancelled the run mid-cycle (the orchestrator's deferred checkpoint + lease
// release have already run) and emits the loop result. Callers poll ctx.Err()
// BEFORE the cycle error so a cancellation reads as a clean signal stop, not a
// confusing "context canceled" cycle error. A signal racing a clean cycle
// completion (a ~µs window) only yields a harmless extra --resume hint: the
// next fresh run finds no unfinished cycle (it was finalized) and proceeds.
func emitSignalStop(stdout, stderr io.Writer, lr *loopResult, cycle int) {
	fmt.Fprintf(stderr, "[loop] received interrupt (SIGINT/SIGTERM) at cycle %d — checkpointed; resume with: evolve loop --resume\n", cycle)
	lr.StopReason = "signal"
	lr.emit(stdout)
}

// runLoop is the `evolve loop` entry point. It parses once, resolves chain
// mode (CLI parameter ORed with policy.json chain.enabled), and then either
// runs exactly ONE batch — byte-identical to the pre-chain contract — or hands
// off to the outer chaining loop in cmd_loop_chain.go. Chaining is never
// entered implicitly: an absent flag and an absent policy block both leave it
// off.
func runLoop(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, rc := parseLoopArgs(args, stderr)
	if rc != 0 {
		return rc
	}
	chainCfg := loadChainConfig(cfg.EvolveDir)
	cfg.ChainMode = cfg.ChainMode || chainCfg.Enabled

	// --dry-run only prints the resolved config (which now carries chain_mode),
	// and --resume is a single-cycle protocol that reads its own checkpoint —
	// neither chains. Say so rather than silently ignoring the operator's ask.
	if cfg.ChainMode && cfg.Resume {
		fmt.Fprintln(stderr, "[chain] --resume runs a single batch; chaining is not applied to a resumed cycle — relaunch with --until-inbox-empty once it completes")
	}
	if cfg.ChainMode && !cfg.DryRun && !cfg.Resume {
		return runLoopChain(cfg, chainCfg, stdin, stdout, stderr)
	}
	return runLoopBatch(cfg, stdin, stdout, stderr)
}

// runLoopBatch runs exactly one batch of cycles: the historical runLoop body,
// unchanged apart from taking an already-parsed config (the chain loop reuses
// the SAME config value for every batch, which is what keeps fleet width and
// every other per-batch setting identical across the chain).
func runLoopBatch(cfg loopConfig, _ io.Reader, stdout, stderr io.Writer) int {
	// First-run onboarding nudge (non-blocking; defaults work without setup).
	maybePrintSetupNudge(stderr, cfg.EvolveDir)

	// ADR-0080 S2: report the worktree plane at boot — a loop launched into
	// the PRIMARY checkout shares its tree with the operator console, the
	// exact overlap that killed lanes 1149-1152. Classification failure is
	// non-fatal (an exotic checkout still runs; the tripwire just stays dark).
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

	// Crash-recovery GC, before any cycle runs: reap tmux sessions left by a
	// PRIOR crashed run. The per-run registry reaper cannot — a SIGKILL'd loop
	// never ran its teardown, and its sessions aren't in this run's registry.
	// This is the "the GC still works even when the last pipeline broke"
	// guarantee. Liveness-scoped, so a live concurrent run is never touched.
	gcOrphanSessions("startup", stderr)

	deps := wireOrchestratorDepsFn(cfg.ProjectRoot, cfg.EvolveDir)
	// orch is narrowed to loopCycleRunner so tests can inject a scripted
	// orchestrator (loopOrchOverride) — the real *core.Orchestrator cannot be
	// driven to emit FinalVerdict=FAIL without a faithful phase machine, which
	// would leave the continue-on-fail call site untestable end-to-end. Same
	// package-var test-seam pattern as wireOrchestratorDepsFn. nil override =
	// production: the concrete orchestrator (byte-identical behavior).
	var orch loopCycleRunner = deps.Orchestrator
	if loopOrchOverride != nil {
		orch = loopOrchOverride
	}
	wc := loadWorkflowConfig(cfg.EvolveDir)
	cycleclassify.SetHangClassifier(loadClassifyConfig(cfg.EvolveDir).HangClassifier)

	maintainBatchState(cfg, wc.AutoPrune, stderr)

	// Build per-cycle env map by propagating EVOLVE_* OS env vars then
	// applying dispatcher-derived overrides. Pre-this-fix, only an
	// allowlist of 4 keys made it through, which silently dropped
	// EVOLVE_REQUIRE_INTENT, EVOLVE_SANDBOX_FALLBACK_ON_EPERM, and every
	// other operator-facing flag the CLAUDE.md env-var table documents
	// (source incident: cycle-108 meta-loop ran with intent_required=false
	// despite EVOLVE_REQUIRE_INTENT=1 set in the operator's shell).
	cycleEnv := buildCycleEnv(cfg, os.Environ())
	cycleCtx := buildCycleContext(cfg)

	lr := loopResult{StopReason: "max_cycles", classifyRoot: cfg.ProjectRoot}
	// Bound the batch's cycle window BEFORE any cycle runs: the spine fail-open
	// roll-up folds the committed dossiers of cycles >= this number, which is the
	// only way a fleet batch (whose lane cycles never return a CycleResult to this
	// process) gets counted. A failed read leaves it 0 = unknown, and the roll-up
	// then falls back to the in-memory results rather than folding all of history.
	if last, lerr := readLastCycleNumber(ctx, deps.Storage); lerr == nil {
		lr.batchFirstCycle = last + 1
	}

	// --resume short-circuits the loop: load the checkpoint, run one
	// cycle from the paused phase, then exit. M3 protocol.
	if cfg.Resume {
		return runResumeBatch(ctx, cfg, orch, cycleEnv, cycleCtx, &lr, stdout, stderr)
	}

	lastBeforeGCHook, exitCode, halt := prepareFreshBatch(ctx, cfg, deps, &lr, stdout, stderr)
	if halt {
		return exitCode
	}

	return (&loopBatchCoordinator{
		ctx:              ctx,
		cfg:              cfg,
		deps:             deps,
		orch:             orch,
		cycleEnv:         cycleEnv,
		cycleContext:     cycleCtx,
		result:           &lr,
		workflow:         wc,
		lastBeforeGCHook: lastBeforeGCHook,
		stdout:           stdout,
		stderr:           stderr,
	}).run()
}

// readCarryoverCount returns the number of carryoverTodos in state.json — the
// goal's remaining backlog the planning phases produced. ok is false when the
// file is absent/unreadable/malformed, so the caller skips the completion check
// rather than ever treating an unreadable state as "goal complete".
// isTaskLevelFailure reports whether a cycle classification is a task-level
// failure eligible for ADR-0072 S5 quarantine. Thin forwarder: the rule itself
// lives in internal/cycleoutcome beside the failure seam that acts on it, so
// the loop's quarantine surface and the closeout can never fork on "whose fault
// was this" (transient infra and system breaches take the S3 halt path and
// never quarantine a todo — AC4).
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
// marker at a clean batch-exit boundary (S2, workspace-hygiene-2026-07 plan),
// so the operator no longer has to run `evolve cycle reset --force` before
// every relaunch. It is the clean-exit counterpart to SealCycle: SILENT on a
// no-op (nothing completed, still in progress, or a live owner) and
// best-effort on error — a finalize failure only WARNs, matching every other
// post-batch cleanup call site in this file, and never turns a clean exit
// into a failed one.
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

// wireOrchestratorDepsFn is the test seam for runLoop. Tests
// substitute a stub that returns a fake orchestrator + in-memory
// storage/ledger so the M4 pipeline can be exercised end-to-end
// without spawning real LLM subagents.
