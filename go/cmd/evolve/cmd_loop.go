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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"

	// Blank import: checkpoint's init() registers core.PhaseBoundaryCheckpointer
	// so the orchestrator writes a resumable checkpoint at every phase boundary.
	// Without this the hook stays nil and the feature silently no-ops in production.
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerverify"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
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
	ConsensusAudit    bool `json:"consensus_audit,omitempty"`
	DryRun            bool `json:"dry_run,omitempty"`
	ForceFresh        bool `json:"force_fresh,omitempty"`
	SkipPreflight     bool `json:"skip_preflight,omitempty"`
	SkipPreflightBoot bool `json:"skip_preflight_boot,omitempty"`
	BypassPolicy      bool `json:"bypass_policy,omitempty"`
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

// runLoop implements `evolve loop`.
// loopSignalContext builds the loop's signal-aware context: SIGINT/SIGTERM
// cancel it so the in-flight cycle unwinds gracefully (the orchestrator's
// deferred checkpoint + run-lease release run) instead of the OS default
// disposition — the silent kill that lost cycles 394/395. A package var so
// tests can inject a cancellable context without delivering a real signal.
var loopSignalContext = func(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
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

func runLoop(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	cfg, rc := parseLoopArgs(args, stderr)
	if rc != 0 {
		return rc
	}

	// First-run onboarding nudge (non-blocking; defaults work without setup).
	maybePrintSetupNudge(stderr, cfg.EvolveDir)

	if cfg.DryRun {
		buf, _ := json.MarshalIndent(map[string]any{
			"dry_run": true,
			"config":  cfg,
		}, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		return 0
	}

	// Graceful interruption (F4): a SIGINT/SIGTERM (Ctrl-C, a polite `kill`, or a
	// sibling stopping this run) cancels ctx so the in-flight cycle unwinds and
	// checkpoints — resumable via `evolve loop --resume` — instead of the silent
	// default-disposition kill. SIGKILL can't be trapped; its recovery is the
	// stale-lease auto-reclaim in `evolve cycle reset`.
	ctx, stop := loopSignalContext(context.Background())
	defer stop()

	// Per-run tmux socket (F6): give THIS loop its own bridge tmux server so an
	// external `tmux -L evolve-bridge kill-server` (a sibling session / operator)
	// can't tear down our agent panes mid-cycle. buildCycleEnv below propagates
	// EVOLVE_TMUX_SOCKET (an EVOLVE_* var) to every bridge subprocess, and the
	// in-process reaper + orphan GC read it too, so all target the same socket. A
	// pre-set value (operator override / nested run) is respected.
	if os.Getenv(bridge.TmuxSocketEnv) == "" {
		_ = os.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(os.Getpid()))
	}

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

	// E2: auto-prune expired failedApproaches at dispatcher start.
	// Non-fatal on error — pruning
	// is cosmetic (the failure-adapter already filters expired entries
	// at read time). Pruning AFTER LoadResumeState so a stale resume
	// pointer doesn't get culled mid-resume.
	if wc.AutoPrune {
		statePath := filepath.Join(cfg.EvolveDir, "state.json")
		if pr, err := failurelog.PruneExpired(statePath, time.Now().UTC()); err != nil {
			fmt.Fprintf(stderr, "[loop] auto-prune: %v\n", err)
		} else if pr.Removed > 0 {
			fmt.Fprintf(stderr, "[loop] auto-prune: removed %d expired failedApproaches (%d→%d)\n", pr.Removed, pr.Before, pr.After)
		}
		// Same TTL prune for the structurally-parallel carryoverTodos array,
		// which otherwise grows unboundedly (no removal path pre-cycle-507).
		// Untimestamped legacy entries are kept (age unknown); fail-open.
		if pr, err := failurelog.PruneExpiredCarryoverTodos(statePath, time.Now().UTC()); err != nil {
			fmt.Fprintf(stderr, "[loop] auto-prune: carryover: %v\n", err)
		} else if pr.Removed > 0 {
			fmt.Fprintf(stderr, "[loop] auto-prune: removed %d expired carryoverTodos (%d→%d)\n", pr.Removed, pr.Before, pr.After)
		}
		// One-time backfill: stamp a conservative TTL on legacy (untimestamped)
		// carryoverTodos so the prune above can converge them instead of keeping
		// them forever (the cycle-360 manual-wipe failure mode). Idempotent —
		// already-stamped entries are skipped. Ordered prune → backfill.
		if stamped, err := failurelog.BackfillLegacyCarryoverExpiry(statePath, failurelog.DefaultCarryoverBackfillTTL, time.Now().UTC()); err != nil {
			fmt.Fprintf(stderr, "[loop] auto-prune: carryover backfill: %v\n", err)
		} else if stamped > 0 {
			fmt.Fprintf(stderr, "[loop] auto-prune: backfilled expiresAt on %d legacy carryoverTodos\n", stamped)
		}
		// Bump cycles_unpicked on every carryover todo that survived to this boot
		// so the advisor's staleness signal is real. Ordered LAST (prune →
		// backfill → increment) so a todo removed this boot isn't counted, and a
		// fresh todo recordFailureLearning writes later this cycle starts at 0.
		if inc, err := failurelog.IncrementCarryoverUnpicked(statePath); err != nil {
			fmt.Fprintf(stderr, "[loop] auto-prune: carryover unpicked: %v\n", err)
		} else if inc > 0 {
			fmt.Fprintf(stderr, "[loop] auto-prune: incremented cycles_unpicked on %d carryoverTodos\n", inc)
		}
	}

	// Gap #5: --reset operator unblock. Prunes the three classifications
	// that accumulate when infra/config issues bricked prior batches:
	// infrastructure-systemic (host-broken / 7d retention), infrastructure
	// -transient (network blips / 1d retention), ship-gate-config (audit
	// PASS but ship-gate refused / 1d retention). Bash dispatcher source:
	// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:749-790.
	// Operator-driven; logs loudly so the choice is auditable.
	if cfg.Reset {
		statePath := filepath.Join(cfg.EvolveDir, "state.json")
		resetClasses := []failurelog.Classification{
			failurelog.InfrastructureSystemic,
			failurelog.InfrastructureTransient,
			failurelog.ShipGateConfig,
		}
		if pr, err := failurelog.PruneByClassification(statePath, resetClasses); err != nil {
			fmt.Fprintf(stderr, "[loop] --reset: %v\n", err)
		} else if pr.Removed > 0 {
			fmt.Fprintf(stderr, "[loop] --reset: pruned %d failedApproaches (infrastructure-{systemic,transient} + ship-gate-config) (%d→%d)\n", pr.Removed, pr.Before, pr.After)
		}
	}

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

	// --resume short-circuits the loop: load the checkpoint, run one
	// cycle from the paused phase, then exit. M3 protocol.
	if cfg.Resume {
		lr.Resumed = true
		rp, err := core.LoadResumeState(ctx, cfg.ProjectRoot, cfg.EvolveDir, core.ResumeOptions{})
		if err != nil {
			fmt.Fprintf(stderr, "evolve loop: resume: %v\n", err)
			lr.StopReason = "error"
			lr.emitFatal(stdout, stderr, cfg, 0)
			return 2
		}
		fmt.Fprintf(stderr, "[resume] cycle=%d phase=%s reason=%s cost=$%.2f\n",
			rp.CycleID, rp.Phase, rp.Reason, rp.CostAtPause)
		req := core.CycleRequest{
			ProjectRoot:           cfg.ProjectRoot,
			GoalHash:              cfg.GoalHash,
			Env:                   cycleEnv,
			Context:               cycleCtx,
			DisableWorkspaceGuard: disableWorkspaceGuardForTest,
			BypassPolicy:          cfg.BypassPolicy,
		}
		result, err := orch.RunCycleFromPhase(ctx, req, rp)
		reapCycleSessions(cfg.ProjectRoot, result.Cycle, stderr)
		lr.Cycles = append(lr.Cycles, result)
		if ctx.Err() != nil {
			emitSignalStop(stdout, stderr, &lr, result.Cycle)
			return 130
		}
		if err != nil {
			lr.StopReason = "error"
			fmt.Fprintf(stderr, "evolve loop: resume cycle %d: %v\n", result.Cycle, err)
		} else if result.FinalVerdict == core.VerdictFAIL {
			lr.StopReason = "fail"
		} else {
			lr.StopReason = "resumed_complete"
		}
		if lr.StopReason == "error" || lr.StopReason == "fail" {
			lr.emitFatal(stdout, stderr, cfg, result.Cycle)
			return 2
		}
		finalizeCompletedCycle(cfg, stderr)
		lr.emit(stdout)
		return 0
	}

	// Boot-time self-heal (cycle 507 F1 wiring): quarantine leaked tracked-source
	// dirt, auto-seal a stranded dead-owner cycle marker, and flag a ship-binary
	// SHA mismatch — BEFORE the unfinished-cycle guard and readiness gate, so a
	// dirty/stranded tree heals rather than wedging the first cycle's tree-diff
	// guard. Fail-open: a recovery error WARNs but never halts the batch.
	// A WITHIN-version ship-SHA mismatch is SELF_SHA_TAMPERED (verifySelfSHA's
	// terminal-gate verdict) — boot-knowable and cycle-fatal. HALT here, pre-scout,
	// so no cycle spends a ~32-40 min lane + LLM budget on a ship doomed from boot
	// (8 cycles wasted, 625-634). The operator recipe already reached stderr.
	if br := bootRecoverFn(ctx, cfg, deps.Ledger, stderr); br.HaltSelfSHA {
		lr.StopReason = "self_sha_boot_halt"
		lr.emit(stdout)
		return 2
	}

	// Unfinished-cycle guard (fresh runs only — resume returned above). A
	// stuck cycle whose number is ahead of lastCycleNumber must not be
	// silently clobbered: that would lose its history. Force the operator to
	// choose — continue it (--resume) or seal it (evolve cycle reset, which
	// archives it for analysis and advances the number). --force-fresh
	// restores the prior silent-clobber behavior as an escape hatch.
	if !cfg.ForceFresh {
		cs, csErr := deps.Storage.ReadCycleState(context.Background())
		last, _ := readLastCycleNumber(context.Background(), deps.Storage)
		// An UNREADABLE cycle-state (truncated JSON from a SIGKILL'd dispatcher
		// mid-write — the most common stuck-cycle cause) must also block: a
		// swallowed parse error would yield a zero CycleState that the
		// predicate treats as "no cycle", letting the fresh run clobber it.
		if csErr != nil || unfinishedCycle(cs, last) {
			// F1-sibling: if the unfinished cycle's run owner is still LIVE (fresh
			// lease AND alive pid), a running loop owns it — steer to attach/wait,
			// never to `evolve cycle reset` (sealing a running cycle is the
			// cycle-395 race; reset would refuse anyway). A dead owner with a still-
			// fresh heartbeat, or a stale/absent lease, falls through to the normal
			// resume-or-seal guidance below rather than wedging the operator at
			// owned_by_live_run against a run that will never come back.
			if csErr == nil && cs.WorkspacePath != "" {
				if lease, ok, _ := runlease.Read(cs.WorkspacePath); ok && runlease.OwnerLive(lease, time.Now(), 0, pidAlive) {
					fmt.Fprintf(stderr, "[loop] cycle %d is owned by a LIVE run (pid %d, lease heartbeat fresh) — another evolve loop is already running it.\n", cs.CycleID, lease.OwnerPID)
					fmt.Fprintln(stderr, "[loop]   • continue/attach:  evolve loop --resume")
					fmt.Fprintln(stderr, "[loop]   • or let it finish — do NOT `evolve cycle reset` or `pkill` a live run (Ctrl-C lets it checkpoint).")
					lr.StopReason = "owned_by_live_run"
					lr.emit(stdout)
					return 2
				}
			}
			if csErr != nil {
				fmt.Fprintf(stderr, "[loop] cycle-state.json is unreadable (%v) — treating as an unfinished cycle to avoid clobbering history.\n", csErr)
			} else {
				fmt.Fprintf(stderr, "[loop] unfinished cycle %d detected at phase %q (lastCycleNumber=%d).\n", cs.CycleID, cs.Phase, last)
			}
			fmt.Fprintln(stderr, "[loop]   • continue it:    evolve loop --resume")
			fmt.Fprintln(stderr, "[loop]   • seal & move on: evolve cycle reset   (archives the cycle for analysis, advances the number)")
			fmt.Fprintln(stderr, "[loop]   (or pass --force-fresh to start fresh and overwrite — history NOT sealed)")
			lr.StopReason = "unfinished_cycle"
			lr.emit(stdout)
			return 2
		}
	}

	// Pre-batch readiness gate (deterministic; NOT an LLM phase — an env-check
	// agent would run THROUGH the very bridge it must verify). Confirms the
	// pipeline can actually run — wiring, profiles, LLM CLIs, host capabilities,
	// and a REAL bridge boot — before any cycle spends LLM budget, catching the
	// cycle-258 ExitREPLBootTimeout at batch start instead of ~30 min in.
	// --skip-preflight bypasses the whole gate; --skip-preflight-boot runs the cheap
	// checks but skips the boot test (CI/offline). No cycle exists
	// yet, so this uses plain emit (cycle=0), mirroring the unfinished-cycle guard.
	if loopPreflightHalts(cfg, stderr) {
		lr.StopReason = "preflight_failed"
		lr.emit(stdout)
		return 2
	}

	lastBeforeGCHook, _ := readLastCycleNumber(context.Background(), deps.Storage)
	runGCHook(cfg, cycleWorkspace(cfg.ProjectRoot, lastBeforeGCHook+1), stderr)

	dc := loadDispatchConfig(cfg.EvolveDir)
	dispPolicy := resolveDispatchPolicy(dc.Policy, stderr)
	threshold := resolveCircuitBreakerThreshold(dc.RepeatThreshold)

	// Circuit-breaker state. PREV_RAN_CYCLE tracks the cycle number
	// returned by the most-recent RunCycle; SAME_CYCLE_STREAK counts
	// consecutive identical values. Trips at threshold.
	prevRanCycle := -1
	sameCycleStreak := 0

	// Consecutive-verdict-FAIL breaker.
	// Default 1 ⇒ stop on the first FAIL (pre-flag contract); >1 lets the
	// batch absorb isolated work-quality misses so a 3-PASS streak can form,
	// while the streak cap still halts a genuinely broken run.
	maxConsecutiveFails := wc.MaxConsecutiveFails
	consecutiveFails := 0

	// Advisor-decided cycle budget. Off ⇒ the operator's
	// --max-cycles governs (byte-identical to today). Enforce with no explicit
	// --max-cycles ⇒ the ceiling becomes the safety cap and per-cycle completion
	// (backlog drained) drives the early stop; advisory computes + logs the
	// would-stop without changing behavior.
	budgetStage := cyclebudget.ParseStage(wc.CycleBudget)
	effectiveMax := cfg.MaxCycles
	if budgetStage == cyclebudget.Enforce && !cfg.MaxCyclesExplicit {
		effectiveMax = wc.MaxCyclesCap
		fmt.Fprintf(stderr, "[loop] no --cycles: advisor-decided, completion-driven (stops when the backlog drains), safety cap=%d\n", effectiveMax)
	}

	// FLEET-AS-POLICY S2: the batch-start snapshot — wave 0's baseline. Every
	// iteration re-resolves the committed fleet block via
	// reloadFleetConfigAtWaveBoundary below (cycle 739), so operator
	// count/min_lanes directives committed mid-batch take effect at the next
	// wave without a lane-killing bounce. Count==1 (absent block, the default)
	// keeps iterations on the existing sequential path below — shouldRunWave
	// gates the wave branch off entirely, so no Supervisor is ever constructed.
	fleetCfg := loadFleetConfig(cfg.EvolveDir)
	for _, w := range fleetCfg.Warnings {
		fmt.Fprintf(stderr, "[loop] WARN: fleet: %s\n", w)
	}
	var waveBinPath string
	if shouldRunWave(fleetCfg) || shouldRunPool(fleetCfg) {
		if bp, err := os.Executable(); err == nil {
			waveBinPath = bp
		} else {
			fmt.Fprintf(stderr, "[loop] WARN: fleet: cannot resolve binary for fleet dispatch, staying sequential: %v\n", err)
			fleetCfg.Count = 1
		}
	}

	// Fleet work-supply-starvation observer (L3 leg of the
	// fleet-concurrency-respect architecture): ONE tracker held across the batch
	// loop so a starved streak spans waves. It advances only on the wave-ran path
	// below; a sequential (Count==1) batch never touches it.
	var starvationTracker fleet.StarvationTracker

	// goal-stall escalation (cmd_loop_goalstall.go): counts consecutive
	// empty/blocked (non-shipping) cycles on the running goal so a goal that keeps
	// landing nothing (cycles 640-644) is escalated, not re-dispatched forever.
	// Held outside the loop so the streak spans cycles. Threshold/weight sourced
	// from policy.json (never a Go literal).
	var goalStall goalStallTracker
	goalStallThreshold, goalStallWeight := loadGoalStallConfig(cfg.EvolveDir)

	// Pipeline-blocker breaker scope: only failures from THIS batch count
	// (digests of cycles > batchStartCycle), so a historic blocker that was
	// already fixed can never halt a fresh healthy run.
	batchStartCycle, _ := readLastCycleNumber(context.Background(), deps.Storage)

	for i := 0; i < effectiveMax; i++ {
		// A SIGINT/SIGTERM that lands between cycles stops cleanly here.
		if ctx.Err() != nil {
			fmt.Fprintf(stderr, "[loop] received interrupt (SIGINT/SIGTERM) before cycle %d — stopping; resume with: evolve loop --resume\n", i+1)
			lr.StopReason = "signal"
			lr.emit(stdout)
			return 130
		}
		// Pipeline-blocker breaker (ADR-0072 extension): before dispatching
		// ANOTHER cycle, check whether this batch's failure digests show a
		// recurring blocker signature (guard-abort class, or one identical
		// fingerprint over ceiling). A blocker poisons every following cycle —
		// halt and escalate NOW rather than burn the rest of the batch
		// (batch-5 lost six cycles to one class with every signal on disk).
		if rc, halted := blockerBreakerHalt(cfg.EvolveDir, cfg.ProjectRoot, batchStartCycle, stderr); halted {
			lr.StopReason = "pipeline_blocker_halt"
			lr.emitFatal(stdout, stderr, cfg, 0)
			return rc
		}
		// CLI-health canary (the per-cycle health seam): one cheap live probe
		// per EXPIRED bench — recovered families rejoin dispatch, still-walled
		// ones re-bench with a doubled cooldown, instead of a full phase
		// re-discovering the wall the expensive way (cycle-283).
		runCLIHealthCanary(cfg.ProjectRoot, cycleEnv, defaultLiveProbe(cfg.ProjectRoot, stderr), stderr)
		// Proactive usage probe (opt-in): concurrently ask each installed family
		// for its quota standing and bench the capped ones BEFORE the first
		// phase boots one — so a fresh cap costs zero wasted boots. Off unless
		// policy.json cli_health.proactive_probe is set.
		runUsageProbe(cfg.ProjectRoot, cfg.EvolveDir, cycleEnv, stderr)

		// fleet-config-hot-reload-wave-boundary (cycle 739): re-resolve the
		// committed fleet block at every wave boundary, before quota/budget
		// sizing. A malformed/unreadable policy.json holds the previous width
		// (never collapses to defaults). A widening from Count==1 mid-batch
		// needs the dispatch binary too, so resolve it here if not yet held.
		fleetCfg = reloadFleetConfigAtWaveBoundary(cfg.EvolveDir, fleetCfg, stderr)
		if (shouldRunWave(fleetCfg) || shouldRunPool(fleetCfg)) && waveBinPath == "" {
			if bp, err := os.Executable(); err == nil {
				waveBinPath = bp
			} else {
				fmt.Fprintf(stderr, "[loop] WARN: fleet: cannot resolve binary for fleet dispatch, staying sequential: %v\n", err)
				fleetCfg.Count = 1
			}
		}

		// FLEET-AS-POLICY S2 wave path: this iteration IS a wave (--max-cycles
		// counts waves). Each lane runs its own full `evolve cycle run`
		// subprocess (own ship/audit/ledger, serialized on the existing
		// .evolve/ship.lock), so a successful wave simply advances to the next
		// iteration — it does not append to lr.Cycles or the sequential
		// breaker/failurelog bookkeeping below, which is scoped to the single
		// in-process orch.RunCycle path. A wave-plan/adapter error, or a
		// triage plan that committed zero lanes (D1: empty-plan guard),
		// WARNs and falls through to that unchanged sequential path for this
		// iteration instead of silently consuming it.
		// FLEET-AS-POLICY L5 rolling-pool path (cycle-553): when the operator opted
		// into policy.fleet.scheduling=="pool", this iteration rolls the backlog
		// through fleet.RunPool (backfills a replacement lane the instant one exits)
		// instead of the wave barrier. Same isolated launch seam (execCycleLaunch)
		// and S3 preflight as the wave path; an empty backlog (or a control-plane
		// refusal) falls through to the sequential path below unchanged. Mutually
		// exclusive with shouldRunWave, so this branch and the wave branch never
		// both fire for one iteration.
		if shouldRunPool(fleetCfg) {
			poolLaunch := execCycleLaunch(waveBinPath, false, cfg.ProjectRoot, cfg.GoalHash, cfg.GoalText, stdout, stderr)
			ran, _, results, perr := dispatchPoolIteration(ctx, fleetCfg, productionWavePreflight(cfg.ProjectRoot), productionPoolPlanFn(cfg, deps.Storage, fleetCfg.Count, stderr), poolLaunch, i)
			switch {
			case perr != nil:
				fmt.Fprintf(stderr, "[loop] WARN: fleet: pool %d dispatch failed, falling back to sequential: %v\n", i, perr)
			case ran:
				failedLanes := 0
				for _, r := range results {
					if r.Err != nil || r.ExitCode != 0 {
						failedLanes++
					}
				}
				fmt.Fprintf(stderr, "[loop] pool %d: %d/%d lanes ok (rolling, target=%d)\n", i, len(results)-failedLanes, len(results), fleetCfg.Count)
				// ADR-0072 (adr0072-fleet-pool-halt-unwired): mirror the wave branch
				// below — a lane that exited with the system-failure halt code forged a
				// verdict, so STOP the batch instead of rolling the next pool iteration
				// (which would only reproduce the fault). The lane subprocess already
				// filed .evolve/pipeline-escalation.json + a P0 pipeline-repair inbox
				// item; ordinary lane FAILs above keep the never-stop retry semantics.
				if rc, sr, halt := dispatchHaltDecision(results); halt {
					fmt.Fprintf(stderr, "[loop] SYSTEM-FAILURE HALT: a fleet lane in pool %d exited with the ADR-0072 halt code (rc=%d) — the lane already filed .evolve/pipeline-escalation.json + a P0 pipeline-repair inbox item. Stopping the batch; diagnose the pipeline (not the task) before resuming with evolve loop --resume.\n", i, systemFailureHaltExitCode)
					lr.StopReason = sr
					lr.emitFatal(stdout, stderr, cfg, 0)
					return rc
				}
				// Escalation boundary (failure-disposition-router S4) — same
				// contract as the wave branch: apply staged intents only once
				// the pool iteration's lanes have drained.
				applyEscalationBoundary(cfg.EvolveDir, i, stderr)
				continue
			default:
				fmt.Fprintf(stderr, "[loop] WARN: fleet: pool %d planned zero lanes (empty backlog), falling back to sequential\n", i)
			}
		}

		if shouldRunWave(fleetCfg) {
			// FLEET-AS-POLICY S3(b) + Q4 budget: quota-aware capacity — active
			// clihealth benches shrink this wave's lane count (copy; min 1), and
			// when a fleet.budget block is present the (fail-open) live quota +
			// pace measurement sizes it further via fleetbudget.Plan (shadow
			// logs, enforce applies + paces). Shrinking to a single lane un-gates
			// shouldRunWave inside dispatchIteration and this iteration falls
			// through to the sequential path below.
			waveCfg, wavePace := budgetAwareWaveConfig(ctx, fleetCfg, cfg.ProjectRoot, cfg.EvolveDir, deps.Storage, stderr)
			launcher := productionWaveLauncher(waveCfg, waveBinPath, cfg.ProjectRoot, cfg.GoalHash, cfg.GoalText, stdout, stderr)
			ran, _, results, werr := dispatchIteration(ctx, waveCfg, productionWavePreflight(cfg.ProjectRoot), productionWavePlanFn(cfg, deps.Storage, waveCfg.Count), launcher, consoleRoutedResolver(cfg.ProjectRoot, stderr), i)
			switch {
			case werr != nil:
				fmt.Fprintf(stderr, "[loop] WARN: fleet: wave %d dispatch failed, falling back to sequential: %v\n", i, werr)
			case ran:
				failedLanes := 0
				for _, r := range results {
					if r.Err != nil || r.ExitCode != 0 {
						failedLanes++
					}
				}
				fmt.Fprintf(stderr, "[loop] wave %d: %d/%d lanes ok\n", i, len(results)-failedLanes, len(results))
				// ADR-0072 (adr0072-fleet-halt-unwired): a lane that exited with the
				// system-failure halt code means the pipeline forged a verdict — the
				// lane subprocess already wrote .evolve/pipeline-escalation.json + a P0
				// pipeline-repair inbox item via haltOnSystemFailure. A forged verdict
				// makes the pipeline untrustworthy fleet-wide, so STOP the batch instead
				// of dispatching the next wave (which would only reproduce the fault).
				// Ordinary lane FAILs above keep the never-stop retry semantics.
				if rc, sr, halt := dispatchHaltDecision(results); halt {
					fmt.Fprintf(stderr, "[loop] SYSTEM-FAILURE HALT: a fleet lane in wave %d exited with the ADR-0072 halt code (rc=%d) — the lane already filed .evolve/pipeline-escalation.json + a P0 pipeline-repair inbox item. Stopping the batch; diagnose the pipeline (not the task) before resuming with evolve loop --resume.\n", i, systemFailureHaltExitCode)
					lr.StopReason = sr
					lr.emitFatal(stdout, stderr, cfg, 0)
					return rc
				}
				// Work-supply-starvation observation: DesiredLanes is the
				// operator-asserted fleetCfg.Count (NOT the quota-shrunk
				// waveCfg.Count); QuotaShrunk distinguishes a benched-family
				// capacity shrink from a dry work supply. On the K-th consecutive
				// starved wave, self-file one weighted inbox todo naming the cause
				// (best-effort: a write failure WARNs and never breaks dispatch).
				obs := fleet.WaveObservation{
					DesiredLanes:  fleetCfg.Count,
					RealizedLanes: len(results),
					QuotaShrunk:   waveCfg.Count < fleetCfg.Count,
				}
				if starvationTracker.Observe(obs, fleetCfg.StarvationK) {
					item := fleet.BuildStarvationItem(obs, fleetCfg.StarvationK, fleetCfg.StarvationWeight, i, time.Now().UTC().Format(time.RFC3339))
					if p, werr := item.WriteTo(cfg.EvolveDir); werr != nil {
						fmt.Fprintf(stderr, "[loop] WARN: fleet: could not self-file starvation todo: %v\n", werr)
					} else {
						fmt.Fprintf(stderr, "[loop] fleet: work-supply starvation after %d waves — self-filed %s\n", fleetCfg.StarvationK, p)
					}
				}
				// Escalation boundary (failure-disposition-router S4): the wave
				// has drained (dispatchIteration blocks on its lanes), so this is
				// the safe moment to mutate the inbox — mid-flight it would race
				// inboxmover.Claim's os.Rename.
				applyEscalationBoundary(cfg.EvolveDir, i, stderr)
				// Enforce-mode budget pacing: idle the affordable inter-wave gap
				// before the next wave (0 in shadow / no floor pressure).
				paceBeforeNextWave(ctx, wavePace, stderr)
				continue
			default:
				// Min-width repair (cycle-547, fleet-min-width-lane-fallback): the
				// wave shrank below shouldRunWave's Count>1 gate (quota bench /
				// budget) so dispatchIteration reported ran=false — but the operator's
				// fleet.count wanted >1 lanes. Rather than drop to width ZERO (the
				// leak-prone process-cwd sequential path), drive up to ONE disjoint
				// candidate through the SAME isolated-worktree launcher, capped at a
				// single lane. Only a genuinely empty backlog still falls back to true
				// sequential. minWidthRepair (cmd_loop_wave.go) owns the guard +
				// WARN-vs-dispatch branching so the call-site wiring is unit-testable.
				oneLauncher := productionWaveLauncher(fleetCfg, waveBinPath, cfg.ProjectRoot, cfg.GoalHash, cfg.GoalText, stdout, stderr)
				if minWidthRepair(ctx, fleetCfg, waveCfg, productionWavePreflight(cfg.ProjectRoot), productionWavePlanFn(cfg, deps.Storage, fleetCfg.Count), oneLauncher, consoleRoutedResolver(cfg.ProjectRoot, stderr), i, stderr) {
					continue
				}
			}
		}

		// Snapshot state.LastCycleNumber so we can detect
		// counter-non-advance after RunCycle returns.
		lastBefore, _ := readLastCycleNumber(context.Background(), deps.Storage)

		req := core.CycleRequest{
			ProjectRoot:           cfg.ProjectRoot,
			GoalHash:              cfg.GoalHash,
			Env:                   cycleEnv,
			Context:               cycleCtx,
			DisableWorkspaceGuard: disableWorkspaceGuardForTest,
			BypassPolicy:          cfg.BypassPolicy,
		}
		result, err := orch.RunCycle(ctx, req)
		reapCycleSessions(cfg.ProjectRoot, result.Cycle, stderr)
		lr.Cycles = append(lr.Cycles, result)
		if ctx.Err() != nil {
			emitSignalStop(stdout, stderr, &lr, result.Cycle)
			return 130
		}
		if err != nil {
			var clf *core.ErrCycleLevelFailure
			if errors.As(err, &clf) {
				fmt.Fprintf(stderr, "evolve loop: cycle %d: %v\n", result.Cycle, err)
				lr.RecoverableFailures++

				statePath := filepath.Join(cfg.EvolveDir, "state.json")
				runsParent := filepath.Join(cfg.ProjectRoot, ".evolve", "runs")
				class := cycleclassify.Classify(cycleWorkspace(cfg.ProjectRoot, result.Cycle))
				_, recordErr := failurelog.Record(statePath, runsParent, failurelog.RecordRequest{
					Cycle:          result.Cycle,
					Classification: string(class.Class),
					ReportPath:     filepath.Join(cycleWorkspace(cfg.ProjectRoot, result.Cycle), "orchestrator-report.md"),
					Now:            time.Now().UTC(),
				})
				if recordErr != nil && !errors.Is(recordErr, failurelog.ErrStateMissing) {
					fmt.Fprintf(stderr, "[loop] WARN: could not record cycle failure: %v\n", recordErr)
				}
				// Release all claimed inbox items back to inbox root on cycle failure
				// so the next batch re-triages them (prevents permanent orphaning).
				// ADR-0072 S5: a task that has now failed task_retry_ceiling times at
				// the TASK level (build/audit/ship-gate) is quarantined instead of
				// re-released, so a poison todo stops being re-picked every cycle.
				// Transient infra and system/kernel breaches are NOT the task's fault
				// (they take the S3 halt path), so they never quarantine here (AC4).
				failPol := policy.DefaultSystemFailurePolicy()
				if pol, polErr := policy.Load(filepath.Join(cfg.EvolveDir, "policy.json")); polErr == nil {
					if fp, fpErr := pol.FailurePolicyConfig(); fpErr == nil {
						failPol = fp
					}
				}
				systemLevel := !isTaskLevelFailure(class.Class)
				if _, relErr := inboxmover.ReleaseCycleProcessingWithQuarantine(inboxmover.Options{
					ProjectRoot: cfg.ProjectRoot,
					Stderr:      stderr,
				}, result.Cycle, "cycle-failure-release", failPol.Thresholds.TaskRetryCeiling, systemLevel); relErr != nil {
					fmt.Fprintf(stderr, "[loop] WARN: could not release cycle %d inbox claims: %v\n", result.Cycle, relErr)
				}
				// All-families quota exhaustion (cycle-656): the dispatch seam
				// wrote a quota-likely checkpoint before aborting. Continuing
				// the batch would burn the next cycle (and its failure retro)
				// into the same drained quota — stop with the same rc=5
				// resumable contract as the post-cycle detector below.
				if errors.Is(err, core.ErrAllFamiliesExhausted) {
					if qp, ok := detectQuotaPause(cfg.EvolveDir); ok {
						fmt.Fprintf(stderr, "QUOTA-PAUSE: cycle=%d wake-at=%s source=%s attempts=%d/%d (all CLI families exhausted mid-cycle)\n",
							qp.Cycle, qp.WakeAt, qp.Source, qp.Attempts, qp.MaxAttempts)
					} else {
						fmt.Fprintf(stderr, "QUOTA-PAUSE: cycle=%d (all CLI families exhausted mid-cycle; checkpoint block missing — resume re-runs from last boundary)\n", result.Cycle)
					}
					fmt.Fprintln(stderr, "[loop]   resume when quota resets: evolve loop --resume")
					lr.StopReason = "quota-pause"
					lr.emit(stdout)
					return 5
				}
				continue
			}
			lr.StopReason = "error"
			fmt.Fprintf(stderr, "evolve loop: cycle %d: %v\n", result.Cycle, err)
			break
		}

		// Resolve ran_cycle the same way bash does: prefer the
		// post-RunCycle state.LastCycleNumber when it advanced; fall
		// back to lastBefore+1 otherwise (and emit counter-non-advance).
		lastAfter, _ := readLastCycleNumber(context.Background(), deps.Storage)
		ranCycle := result.Cycle
		workspace := cycleWorkspace(cfg.ProjectRoot, ranCycle)

		// ADR-0072: a SYSTEM-level failure (the pipeline forged a verdict, not a
		// task-code failure) HALTS the loop for diagnosis instead of retrying the
		// same inbox task — which would only reproduce the fault (the cycle
		// 862→899 livelock). This Go floor is non-negotiable. Checked HERE, before
		// the quota-pause / circuit-breaker / verify branches below can early-
		// return, so the escalation dossier + P0 pipeline-repair item are always
		// written even when another condition also trips this cycle. rc=4 is
		// distinct from the soft rc=3 (batch completed with absorbed FAILs).
		if sf := result.SystemFailure; sf != nil && sf.Halt {
			rc := haltOnSystemFailure(cfg.EvolveDir, cfg.ProjectRoot, ranCycle, workspace, sf, stderr)
			lr.StopReason = "system_failure_halt"
			lr.emitFatal(stdout, stderr, cfg, ranCycle)
			return rc
		}

		if lastAfter <= lastBefore {
			// The counter didn't advance — record an abnormal event in
			// the cycle workspace if it exists. (Workspace may be
			// absent on early-cycle errors; emit is best-effort.)
			if dirExists(workspace) {
				w := dispatchevents.NewWriter(workspace)
				_ = w.EmitCounterNonAdvance(ranCycle)
			}
			fmt.Fprintf(stderr, "[loop] NOTE: lastCycleNumber did not advance after cycle %d — verdict likely WARN/FAIL\n", ranCycle)
		}

		// Cost telemetry (display-only). Sum the cycle's per-phase costs and add
		// to the batch total for the JSON output's total_cost_usd field. Cost no
		// longer gates anything — no cap, no checkpoint-by-cost, no budget stop
		// (the token-budget cost calculation was unreliable across LLM models:
		// tmux/subscription claude reports $0, gemini was hardcoded, ollama is
		// free). Missing-workspace / no-logs is non-fatal.
		if cs, err := cyclecost.SummarizeCycle(workspace, ranCycle); err == nil {
			lr.TotalCost += cs.Total.CostUSD
			fmt.Fprintf(stderr, "[loop] cycle %d cost: $%.4f (batch total: $%.4f)\n",
				ranCycle, cs.Total.CostUSD, lr.TotalCost)
		}

		// Gap #3: QUOTA-PAUSE detection. After each cycle, read
		// cycle-state.json:checkpoint. If enabled=true AND reason==
		// "quota-likely", emit the structured marker line + rc=5 + break.
		// Tested BEFORE budget-cap + verify so quota wall takes priority
		// over downstream checks. Source: bash dispatcher lines 907-930.
		if qp, ok := detectQuotaPause(cfg.EvolveDir); ok {
			fmt.Fprintf(stderr, "QUOTA-PAUSE: cycle=%d wake-at=%s source=%s attempts=%d/%d\n",
				qp.Cycle, qp.WakeAt, qp.Source, qp.Attempts, qp.MaxAttempts)
			fmt.Fprintln(stderr, "[loop]   to auto-resume in-session: SKILL.md / /loop wrapper calls ScheduleWakeup until wake-at then /evo:loop --resume")
			fmt.Fprintln(stderr, "[loop]   to resume manually: evolve loop --resume")
			lr.StopReason = "quota-pause"
			lr.emit(stdout)
			return 5
		}

		// Same-cycle circuit breaker (D4). Bash trips this when
		// run-cycle.sh fails to register a cycle but the dispatcher
		// keeps iterating — the same ran_cycle value comes back over
		// and over. After `threshold` consecutive hits, abort the batch.
		var tripped bool
		prevRanCycle, sameCycleStreak, tripped = updateBreaker(prevRanCycle, sameCycleStreak, ranCycle, threshold)
		if tripped {
			fmt.Fprintf(stderr, "[loop] ABORT: same cycle number (%d) reported %d consecutive times (threshold=%d) — dispatcher deadlocked\n", ranCycle, sameCycleStreak, threshold)
			if dirExists(workspace) {
				w := dispatchevents.NewWriter(workspace)
				_ = w.EmitCircuitBreakerTripped(ranCycle, sameCycleStreak, threshold)
			}
			lr.StopReason = "circuit_breaker"
			lr.emitFatal(stdout, stderr, cfg, ranCycle)
			return 1
		}

		// Verify + classify pipeline (D1 + D2 wired together). Skipped
		// when EVOLVE_DISPATCH_POLICY=off. On verify-fail in `verify`
		// mode, classify + emit event + continue (recoverable classes)
		// or break (integrity-breach). On `stop` mode, any verify-fail
		// halts the batch.
		if dispPolicy != dispatchPolicyOff {
			vc := ledgerverify.LoadVerifyContext(workspace, cfg.EvolveDir)
			vr, vErr := ledgerverify.VerifyCycle(context.Background(), deps.Ledger, ranCycle, ledgerverify.Options(vc))
			if vErr != nil {
				fmt.Fprintf(stderr, "[loop] verify cycle %d: %v\n", ranCycle, vErr)
			} else if !vr.OK {
				// Emit verify-failed event + classify the failure.
				var emitter *dispatchevents.Writer
				if dirExists(workspace) {
					emitter = dispatchevents.NewWriter(workspace)
					_ = emitter.EmitVerifyFailed(ranCycle, vr.Missing)
				}
				class := cycleclassify.Classify(workspace)
				if emitter != nil {
					_ = emitter.EmitClassification(ranCycle, string(class.Class))
				}
				fmt.Fprintf(stderr, "[loop] cycle %d incomplete: missing %v classification=%s\n", ranCycle, vr.Missing, class.Class)

				if dispPolicy == dispatchPolicyStop {
					lr.StopReason = "verify_failed_stop"
					lr.emitFatal(stdout, stderr, cfg, ranCycle)
					return 2
				}
				// policy == verify: STOP only on integrity-breach;
				// recoverable classes continue the loop.
				if class.Class == cycleclassify.ClassIntegrityBreach {
					lr.StopReason = "integrity_breach"
					lr.emitFatal(stdout, stderr, cfg, ranCycle)
					return 2
				}
				// D2: an empty-output session is the subscription-quota-wall
				// signature. Continuing would just burn the next cycle into the
				// same wall, so QUOTA-PAUSE (rc=5, resumable) instead — the same
				// stop semantics as the checkpoint-driven path above, but driven
				// by the classifier since the empty session never wrote a
				// checkpoint. The failure is still recorded below first so the
				// history survives the pause.
				if class.Marker == cycleclassify.MarkerQuotaLikelyEmptyOutput {
					fmt.Fprintf(stderr, "QUOTA-PAUSE: cycle=%d source=%s (empty-output session — likely subscription quota wall)\n", ranCycle, class.Source)
					fmt.Fprintln(stderr, "[loop]   resume when quota resets: evolve loop --resume")
					// Persist the failure so history survives the pause. The
					// stop is unconditional (rc=5 either way) but a Record
					// error is still surfaced — silent swallow would hide a
					// state.json corruption that the next --resume will fail on.
					if _, recErr := failurelog.Record(filepath.Join(cfg.EvolveDir, "state.json"), filepath.Join(cfg.ProjectRoot, ".evolve", "runs"), failurelog.RecordRequest{
						Cycle:          ranCycle,
						Classification: string(class.Class),
						ReportPath:     filepath.Join(workspace, "orchestrator-report.md"),
						Now:            time.Now().UTC(),
					}); recErr != nil {
						fmt.Fprintf(stderr, "[loop] WARN: could not record quota-pause failure for cycle %d: %v\n", ranCycle, recErr)
					}
					lr.StopReason = "quota-pause"
					lr.emit(stdout)
					return 5
				}
				// E1: persist the recoverable failure to
				// state.json:failedApproaches so the next cycle's
				// orchestrator can read history + adapt. Atomic-write
				// failure (state.json unwritable) escalates to a hard
				// halt — the bash equivalent at line 1172-1178 treats
				// this as a silent-deadlock case and aborts. Match the
				// bash exit code 1 (dispatcher infrastructure issue).
				statePath := filepath.Join(cfg.EvolveDir, "state.json")
				runsParent := filepath.Join(cfg.ProjectRoot, ".evolve", "runs")
				_, recordErr := failurelog.Record(statePath, runsParent, failurelog.RecordRequest{
					Cycle:          ranCycle,
					Classification: string(class.Class),
					ReportPath:     filepath.Join(workspace, "orchestrator-report.md"),
					Now:            time.Now().UTC(),
				})
				if recordErr != nil {
					if errors.Is(recordErr, failurelog.ErrStateMissing) {
						// Soft WARN: state.json wasn't initialized by
						// pre-flight. Bash equivalent at line 647-648
						// logs and continues. Treat as recoverable.
						fmt.Fprintf(stderr, "[loop] WARN: state.json missing — cannot persist failed approach for cycle %d\n", ranCycle)
					} else {
						// Hard halt: state.json exists but write
						// failed (EPERM / disk full / parse error).
						// Matches bash line 1172-1178: silent-deadlock
						// case, abort with rc=1.
						fmt.Fprintf(stderr, "[loop] ABORT: state.json unwritable mid-batch (cycle %d): %v\n", ranCycle, recordErr)
						lr.StopReason = "state_unwritable"
						lr.emit(stdout)
						return 1
					}
				}
				lr.RecoverableFailures++
				fmt.Fprintf(stderr, "[loop] RECOVERABLE-FAILURE recorded: cycle=%d classification=%s\n", ranCycle, class.Class)
			}
		}

		var stopOnFail bool
		consecutiveFails, stopOnFail = consecutiveFailBreaker(
			result.FinalVerdict == core.VerdictFAIL, consecutiveFails, maxConsecutiveFails)
		if stopOnFail {
			lr.StopReason = "fail"
			break
		}
		if result.FinalVerdict == core.VerdictFAIL {
			recordAbsorbedFail(cfg, ranCycle, stderr)
			lr.ContinuedFailures++
			fmt.Fprintf(stderr, "[loop] cycle %d verdict=FAIL — continuing (consecutive %d of max %d, workflow policy)\n",
				ranCycle, consecutiveFails, maxConsecutiveFails)
		}

		// Goal-stall escalation: an empty/blocked cycle shipped nothing and left
		// no FAIL signal (the consecutive-FAIL breaker above misses it). N
		// consecutive such cycles on the SAME goal means blind re-dispatch is
		// burning pipelines (goal_hash 805f6ced, cycles 640-644) — self-file a
		// weighted inbox todo naming the goal + reasons and emit an abnormal-event
		// instead of re-running the identical goal again. The queue is never
		// halted: escalate and continue (never_stop_queue_inject_inbox).
		nonShipping := result.FinalVerdict == core.CycleOutcomeSkippedUnknown ||
			result.FinalVerdict == core.CycleOutcomeSkippedAuditAdvisory
		if esc := goalStall.observe(nonShipping, result.FinalVerdict, goalStallThreshold); esc != nil {
			handleGoalStall(cfg.EvolveDir, cfg.GoalHash, workspace, ranCycle, esc, goalStallThreshold, goalStallWeight, stderr)
		}

		// Escalation boundary (failure-disposition-router S4): apply the intents
		// S3 staged during the cycle now that nothing is in flight — the only
		// moment an inbox write cannot race inboxmover.Claim's os.Rename.
		applyEscalationBoundary(cfg.EvolveDir, ranCycle, stderr)

		// Cycle-budget completion: when the operator gave no explicit --cycles,
		// the advisor decides — on a non-FAIL cycle, if the backlog the planning
		// phases produced is drained (or the advisor judged the goal done), stop
		// instead of burning cycles to the cap. An explicit --cycles N is a
		// contract to run exactly N, so it is never early-stopped here. A FAIL
		// cycle is left to the consecutive-fail breaker; an unreadable state.json
		// skips the check (never falsely "complete").
		if budgetStage != cyclebudget.Off && !cfg.MaxCyclesExplicit && result.FinalVerdict != core.VerdictFAIL {
			if backlog, ok := readCarryoverCount(filepath.Join(cfg.EvolveDir, "state.json")); ok {
				d := cyclebudget.Next(budgetStage, i+1, effectiveMax, backlog, false)
				if d.Advisory {
					fmt.Fprintf(stderr, "[loop] cycle-budget ADVISORY: would stop (%s) after cycle %d (backlog=%d)\n", d.Reason, ranCycle, backlog)
				}
				if d.Stop {
					fmt.Fprintf(stderr, "[loop] cycle-budget: stopping (%s) after cycle %d (backlog=%d)\n", d.Reason, ranCycle, backlog)
					lr.StopReason = d.Reason
					break
				}
			}
		}
	}

	if lr.StopReason == "error" || lr.StopReason == "fail" {
		lr.emitFatal(stdout, stderr, cfg, lastCycleIn(lr))
		return 2
	}
	finalizeCompletedCycle(cfg, stderr)
	lr.emit(stdout)
	// E1 exit-code contract: when any cycle in the batch hit a
	// recoverable failure (verify-fail + classify → recoverable) OR a
	// verdict-FAIL the breaker absorbed and continued past, signal rc=3
	// so CI sees "batch completed but with audit/build/infra issues".
	// Matches bash dispatcher's DISPATCH_RC=3.
	if lr.RecoverableFailures > 0 || lr.ContinuedFailures > 0 {
		return 3
	}
	return 0
}

// readCarryoverCount returns the number of carryoverTodos in state.json — the
// goal's remaining backlog the planning phases produced. ok is false when the
// file is absent/unreadable/malformed, so the caller skips the completion check
// rather than ever treating an unreadable state as "goal complete".
// isTaskLevelFailure reports whether a cycle classification is a task-level
// failure eligible for ADR-0072 S5 quarantine. Only genuine per-task defects
// (build/audit/ship-gate) count; transient infrastructure and system/kernel
// breaches are not the task's fault and take the S3 halt path, so they never
// quarantine a todo (AC4 — S3 precedence over task quarantine).
func isTaskLevelFailure(c cycleclassify.Classification) bool {
	switch c {
	case cycleclassify.ClassBuildFail, cycleclassify.ClassAuditFail, cycleclassify.ClassShipGateConfig:
		return true
	default:
		return false
	}
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
