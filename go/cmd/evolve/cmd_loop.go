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
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"

	// Blank import: checkpoint's init() registers core.PhaseBoundaryCheckpointer
	// so the orchestrator writes a resumable checkpoint at every phase boundary.
	// Without this the hook stays nil and the feature silently no-ops in production.
	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerverify"
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
	// (or a positional count). When false and EVOLVE_CYCLE_BUDGET=enforce, the
	// loop defaults its ceiling to the safety cap and lets completion drive the
	// stop, instead of the legacy default of 1.
	MaxCyclesExplicit bool `json:"max_cycles_explicit,omitempty"`
	Resume            bool `json:"resume,omitempty"`
	Reset             bool `json:"reset,omitempty"`
	ConsensusAudit    bool `json:"consensus_audit,omitempty"`
	DryRun            bool `json:"dry_run,omitempty"`
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
		rp, err := core.LoadResumeState(context.Background(), cfg.ProjectRoot, cfg.EvolveDir, core.ResumeOptions{})
		if err != nil {
			fmt.Fprintf(stderr, "evolve loop: resume: %v\n", err)
			lr.StopReason = "error"
			lr.emitFatal(stdout, stderr, cfg, 0)
			return 2
		}
		fmt.Fprintf(stderr, "[resume] cycle=%d phase=%s reason=%s cost=$%.2f\n",
			rp.CycleID, rp.Phase, rp.Reason, rp.CostAtPause)
		req := core.CycleRequest{
			ProjectRoot: cfg.ProjectRoot,
			GoalHash:    cfg.GoalHash,
			Env:         cycleEnv,
			Context:     cycleCtx,
		}
		result, err := orch.RunCycleFromPhase(context.Background(), req, rp)
		reapCycleSessions(cfg.ProjectRoot, result.Cycle, stderr)
		lr.Cycles = append(lr.Cycles, result)
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
		lr.emit(stdout)
		return 0
	}

	// Unfinished-cycle guard (fresh runs only — resume returned above). A
	// stuck cycle whose number is ahead of lastCycleNumber must not be
	// silently clobbered: that would lose its history. Force the operator to
	// choose — continue it (--resume) or seal it (evolve cycle reset, which
	// archives it for analysis and advances the number). EVOLVE_FORCE_FRESH=1
	// restores the prior silent-clobber behavior as an escape hatch.
	if os.Getenv("EVOLVE_FORCE_FRESH") != "1" {
		cs, csErr := deps.Storage.ReadCycleState(context.Background())
		last, _ := readLastCycleNumber(context.Background(), deps.Storage)
		// An UNREADABLE cycle-state (truncated JSON from a SIGKILL'd dispatcher
		// mid-write — the most common stuck-cycle cause) must also block: a
		// swallowed parse error would yield a zero CycleState that the
		// predicate treats as "no cycle", letting the fresh run clobber it.
		if csErr != nil || unfinishedCycle(cs, last) {
			if csErr != nil {
				fmt.Fprintf(stderr, "[loop] cycle-state.json is unreadable (%v) — treating as an unfinished cycle to avoid clobbering history.\n", csErr)
			} else {
				fmt.Fprintf(stderr, "[loop] unfinished cycle %d detected at phase %q (lastCycleNumber=%d).\n", cs.CycleID, cs.Phase, last)
			}
			fmt.Fprintln(stderr, "[loop]   • continue it:    evolve loop --resume")
			fmt.Fprintln(stderr, "[loop]   • seal & move on: evolve cycle reset   (archives the cycle for analysis, advances the number)")
			fmt.Fprintln(stderr, "[loop]   (or set EVOLVE_FORCE_FRESH=1 to start fresh and overwrite — history NOT sealed)")
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
	// EVOLVE_SKIP_PREFLIGHT=1 bypasses the whole gate; EVOLVE_SKIP_PREFLIGHT_BOOT=1
	// runs the cheap checks but skips the boot test (CI/offline). No cycle exists
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

	// Advisor-decided cycle budget (EVOLVE_CYCLE_BUDGET). Off ⇒ the operator's
	// --max-cycles governs (byte-identical to today). Enforce with no explicit
	// --max-cycles ⇒ the ceiling becomes the safety cap and per-cycle completion
	// (backlog drained) drives the early stop; advisory computes + logs the
	// would-stop without changing behavior.
	budgetStage := cyclebudget.ParseStage(os.Getenv("EVOLVE_CYCLE_BUDGET"))
	effectiveMax := cfg.MaxCycles
	if budgetStage == cyclebudget.Enforce && !cfg.MaxCyclesExplicit {
		effectiveMax = wc.MaxCyclesCap
		fmt.Fprintf(stderr, "[loop] cycle-budget=enforce: completion-driven, safety cap=%d (no explicit --max-cycles)\n", effectiveMax)
	}

	for i := 0; i < effectiveMax; i++ {
		// CLI-health canary (the per-cycle health seam): one cheap live probe
		// per EXPIRED bench — recovered families rejoin dispatch, still-walled
		// ones re-bench with a doubled cooldown, instead of a full phase
		// re-discovering the wall the expensive way (cycle-283).
		runCLIHealthCanary(cfg.ProjectRoot, cycleEnv, func(driver string) (int, string, string) {
			probeCtx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			return bridge.LiveSmokeTest(probeCtx, driver,
				&bridge.Config{ProjectRoot: cfg.ProjectRoot}, bridge.Deps{Stderr: stderr})
		}, stderr)

		// Snapshot state.LastCycleNumber so we can detect
		// counter-non-advance after RunCycle returns.
		lastBefore, _ := readLastCycleNumber(context.Background(), deps.Storage)

		req := core.CycleRequest{
			ProjectRoot: cfg.ProjectRoot,
			GoalHash:    cfg.GoalHash,
			Env:         cycleEnv,
			Context:     cycleCtx,
		}
		result, err := orch.RunCycle(context.Background(), req)
		reapCycleSessions(cfg.ProjectRoot, result.Cycle, stderr)
		lr.Cycles = append(lr.Cycles, result)
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
				if _, relErr := inboxmover.ReleaseCycleProcessing(inboxmover.Options{
					ProjectRoot: cfg.ProjectRoot,
					Stderr:      stderr,
				}, result.Cycle); relErr != nil {
					fmt.Fprintf(stderr, "[loop] WARN: could not release cycle %d inbox claims: %v\n", result.Cycle, relErr)
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
			fmt.Fprintln(stderr, "[loop]   to auto-resume in-session: SKILL.md / /loop wrapper calls ScheduleWakeup until wake-at then /evolve-loop --resume")
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

		// Cycle-budget completion: on a non-FAIL cycle, if the backlog the
		// planning phases produced is drained (or the advisor judged the goal
		// done), stop instead of burning cycles to the cap. A FAIL cycle is left
		// to the consecutive-fail breaker; an unreadable state.json skips the
		// check (never falsely "complete").
		if budgetStage != cyclebudget.Off && result.FinalVerdict != core.VerdictFAIL {
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

// wireOrchestratorDepsFn is the test seam for runLoop. Tests
// substitute a stub that returns a fake orchestrator + in-memory
// storage/ledger so the M4 pipeline can be exercised end-to-end
// without spawning real LLM subagents.
