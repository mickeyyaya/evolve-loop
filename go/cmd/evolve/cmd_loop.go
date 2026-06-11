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
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"

	// Blank import: checkpoint's init() registers core.PhaseBoundaryCheckpointer
	// so the orchestrator writes a resumable checkpoint at every phase boundary.
	// Without this the hook stays nil and the feature silently no-ops in production.
	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/faillearn"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/goalhash"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgerverify"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
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
type loopResult struct {
	StopReason          string             `json:"stop_reason"`
	Cycles              []core.CycleResult `json:"cycles"`
	TotalCost           float64            `json:"total_cost_usd"`
	Resumed             bool               `json:"resumed,omitempty"`
	RecoverableFailures int                `json:"recoverable_failures,omitempty"`
	// CycleOutcomes is the R6 SLO classification per cycle (SHIPPED /
	// SALVAGED / FAILED_EXPLAINED / FAILED_UNEXPLAINED), computed from the
	// C1 records at emit time — the batch-level "every cycle delivers a
	// result" accounting the EVOLVE_PHASE_RECOVERY soak reads.
	CycleOutcomes []cycleOutcomeEntry `json:"cycle_outcomes,omitempty"`
	// classifyRoot, when set (the loop entry points set it once), makes
	// emit() populate CycleOutcomes from <root>/.evolve/runs/cycle-N.
	classifyRoot string
}

type cycleOutcomeEntry struct {
	Cycle   int    `json:"cycle"`
	Outcome string `json:"outcome"`
	Detail  string `json:"detail,omitempty"`
}

// emit writes lr to w as the canonical pretty-JSON dispatcher output.
// JSON format byte-identical to the previous inline marshaling — tests
// asserting on stop_reason / total_cost_usd / etc. continue to pass.
//
// Today loopResult only holds string/float64/bool/int/[]CycleResult,
// so MarshalIndent cannot fail. If a future field (channel, func,
// unencodable interface) breaks that, emit a structured error envelope
// instead of a silent empty line so the failure is observable —
// dispatchers and `evolve loop` consumers grep stop_reason.
func (lr *loopResult) emit(w io.Writer) {
	// R6: classify every cycle's ending from its C1 records at the single
	// output chokepoint (every exit path funnels here). A
	// FAILED_UNEXPLAINED additionally self-files an inbox defect — that
	// bucket means a terminal path escaped the C1 chokepoint, which is
	// itself a defect. Best-effort: classification must never break the
	// dispatcher contract.
	if lr.classifyRoot != "" && len(lr.Cycles) > 0 && lr.CycleOutcomes == nil {
		for _, c := range lr.Cycles {
			oc, detail := cyclehealth.ClassifyOutcome(cycleWorkspace(lr.classifyRoot, c.Cycle))
			lr.CycleOutcomes = append(lr.CycleOutcomes, cycleOutcomeEntry{Cycle: c.Cycle, Outcome: string(oc), Detail: detail})
			if oc == cyclehealth.OutcomeFailedUnexplained {
				fileUnexplainedOutcomeDefect(lr.classifyRoot, c.Cycle, detail)
			}
		}
	}
	buf, err := json.MarshalIndent(lr, "", "  ")
	if err != nil {
		fmt.Fprintf(w, `{"stop_reason":"marshal_error","error":%q}`+"\n", err.Error())
		return
	}
	fmt.Fprintln(w, string(buf))
}

// fileUnexplainedOutcomeDefect self-files an inbox item for the alarm
// bucket (R6.3): FAILED_UNEXPLAINED means the "every terminal path records
// its outcome" invariant (ADR-0044 C1) has a hole — exactly what the inbox
// exists to capture. Idempotent per cycle (fixed filename); best-effort.
func fileUnexplainedOutcomeDefect(projectRoot string, cycle int, detail string) {
	dir := filepath.Join(projectRoot, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, fmt.Sprintf("auto-unexplained-outcome-cycle-%d.json", cycle))
	if _, err := os.Stat(path); err == nil {
		return // already filed
	}
	body, err := json.MarshalIndent(map[string]any{
		"id":               fmt.Sprintf("unexplained-outcome-cycle-%d", cycle),
		"action":           fmt.Sprintf("Cycle %d ended FAILED_UNEXPLAINED (%s). Every terminal path must record a ship PASS, a salvage, or an abort_reason (ADR-0044 C1) — locate the escaping path and route it through recordPhaseOutcome.", cycle, detail),
		"priority":         "HIGH",
		"weight":           0.8,
		"evidence_pointer": fmt.Sprintf(".evolve/runs/cycle-%d/phase-timing.json", cycle),
		"injected_at":      time.Now().UTC().Format(time.RFC3339),
		"injected_by":      "loop-outcome-classifier",
	}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, body, 0o644)
}

// emitFatal is emit for ABNORMAL exits: record the loop-fatal learning
// first (failure floor, inbox retro-always-invariant gap 3), then emit.
// Plain emit stays in use for success exits and for paths that already
// recorded their failure (quota-pause empty-output) or structurally
// cannot (state_unwritable — the record write is the thing that failed;
// unfinished-cycle guard — learning is captured downstream by the
// forced reset/resume).
func (lr *loopResult) emitFatal(w, stderr io.Writer, cfg loopConfig, cycle int) {
	recordLoopFatal(stderr, cfg, cycle, lr.StopReason)
	lr.emit(w)
}

// recordLoopFatal persists a batch-level failedApproaches entry
// (classification loop-fatal, stop_reason in the summary) plus a
// deterministic lesson artifact. Best-effort: a floor failure must
// never change the exit path — WARN is the only trace. cycle may be 0
// when unknown (Record's lastCycleNumber advance is monotonic, so a
// zero cycle cannot regress the counter).
func recordLoopFatal(stderr io.Writer, cfg loopConfig, cycle int, stopReason string) {
	now := time.Now().UTC()
	stop := "stop_reason=" + stopReason
	if _, err := failurelog.Record(filepath.Join(cfg.EvolveDir, "state.json"), "", failurelog.RecordRequest{
		Cycle:          cycle,
		Classification: string(failurelog.LoopFatal),
		Summary:        stop,
		Now:            now,
	}); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not record loop-fatal (%s): %v\n", stopReason, err)
	}
	ev := faillearn.FailureEvent{
		Cycle:          cycle,
		FailedPhase:    stop,
		Scope:          faillearn.ScopeLoop,
		Classification: string(failurelog.LoopFatal),
		Verdict:        "FATAL",
		Summary:        fmt.Sprintf("batch stopped abnormally (%s) at cycle %d", stop, cycle),
		Now:            now,
	}
	if err := faillearn.WriteArtifacts(ev, "", filepath.Join(cfg.EvolveDir, "instincts", "lessons")); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: could not write loop-fatal lesson: %v\n", err)
	}
}

// lastCycleIn returns the last attempted cycle number in the batch, 0
// when no cycle ran.
func lastCycleIn(lr loopResult) int {
	if n := len(lr.Cycles); n > 0 {
		return lr.Cycles[n-1].Cycle
	}
	return 0
}

// loopConfig is the resolved invocation. Extracted so --dry-run and
// tests can inspect what would be done without invoking the
// orchestrator.
type loopConfig struct {
	ProjectRoot    string  `json:"project_root"`
	EvolveDir      string  `json:"evolve_dir"`
	GoalHash       string  `json:"goal_hash"`
	GoalText       string  `json:"goal_text,omitempty"`
	Strategy       string  `json:"strategy"`
	MaxCycles      int     `json:"max_cycles"`
	BudgetUSD      float64 `json:"budget_usd"`
	BatchCapUSD    float64 `json:"batch_cap_usd"`
	Resume         bool    `json:"resume,omitempty"`
	Reset          bool    `json:"reset,omitempty"`
	ConsensusAudit bool    `json:"consensus_audit,omitempty"`
	DryRun         bool    `json:"dry_run,omitempty"`
	// BudgetDriven is true when --budget-usd (or --budget alias) is
	// passed with a finite positive value. In budget mode, MaxCycles
	// becomes a safety upper bound (default EVOLVE_BUDGET_MAX_CYCLES=50)
	// and the loop stops when cumulative cost ≥ BudgetUSD with
	// stop_reason=budget rc=0 (mirrors bash v8.60.0 contract).
	BudgetDriven bool `json:"budget_driven,omitempty"`
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
	orch := deps.Orchestrator

	// E2: auto-prune expired failedApproaches at dispatcher start.
	// Opt-out via EVOLVE_AUTO_PRUNE=0. Non-fatal on error — pruning
	// is cosmetic (the failure-adapter already filters expired entries
	// at read time). Pruning AFTER LoadResumeState so a stale resume
	// pointer doesn't get culled mid-resume.
	if os.Getenv("EVOLVE_AUTO_PRUNE") != "0" {
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

	// Pre-emptive checkpoint thresholds. WARN at warnPct (default 80),
	// signal next-cycle to checkpoint at criticalPct (default 95).
	warnPct, criticalPct := resolveCheckpointThresholds()

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
			Budget: core.BudgetEnvelope{
				MaxUSD:      cfg.BudgetUSD,
				BatchCapUSD: cfg.BatchCapUSD,
			},
			Env:     cycleEnv,
			Context: cycleCtx,
		}
		result, err := orch.RunCycleFromPhase(context.Background(), req, rp)
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

	policy := resolveDispatchPolicy(stderr)
	threshold := resolveCircuitBreakerThreshold()

	// Circuit-breaker state. PREV_RAN_CYCLE tracks the cycle number
	// returned by the most-recent RunCycle; SAME_CYCLE_STREAK counts
	// consecutive identical values. Trips at threshold.
	prevRanCycle := -1
	sameCycleStreak := 0
	budgetUnobsWarned := false // cycle-190: warn once when --budget-usd can't gate

	for i := 0; i < cfg.MaxCycles; i++ {
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
			Budget: core.BudgetEnvelope{
				MaxUSD:      cfg.BudgetUSD,
				BatchCapUSD: cfg.BatchCapUSD,
			},
			Env:     cycleEnv,
			Context: cycleCtx,
		}
		result, err := orch.RunCycle(context.Background(), req)
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

		// C3: cost accumulation + checkpoint thresholds. Sum the
		// cycle's per-phase costs from <workspace>/*-stdout.log, add
		// to batch total, then check against batch cap. WARN at
		// warnPct, set EVOLVE_CHECKPOINT_REQUEST=1 for the next cycle
		// at criticalPct. Missing-workspace / no-logs is non-fatal.
		// Gap #7: EVOLVE_BATCH_BUDGET_DISABLE=1 skips ALL cost tracking
		// (parity with bash dispatcher). EVOLVE_CHECKPOINT_DISABLE=1
		// skips the threshold WARN/CRITICAL but keeps cost accounting.
		batchBudgetDisabled := os.Getenv("EVOLVE_BATCH_BUDGET_DISABLE") == "1"
		checkpointDisabled := os.Getenv("EVOLVE_CHECKPOINT_DISABLE") == "1"
		if !batchBudgetDisabled {
			if cs, err := cyclecost.SummarizeCycle(workspace, ranCycle); err == nil {
				lr.TotalCost += cs.Total.CostUSD
				fmt.Fprintf(stderr, "[loop] cycle %d cost: $%.4f (batch total: $%.4f / cap $%.2f)\n",
					ranCycle, cs.Total.CostUSD, lr.TotalCost, cfg.BatchCapUSD)
				// cycle-190: a --budget-usd run whose cycle reports $0 cost cannot
				// be gated by spend (tmux-driver / subscription auth surfaces no
				// usage). Warn ONCE so the operator isn't misled into thinking the
				// budget bounds the run — the cycle cap governs instead.
				if budgetGatingUnobservable(cfg.BudgetDriven, cs.Total.CostUSD) && !budgetUnobsWarned {
					budgetUnobsWarned = true
					fmt.Fprintf(stderr, "[loop] WARN BUDGET-UNOBSERVABLE: --budget-usd $%.2f cannot gate this run — cycle %d reported $0.0000 cost (tmux-driver / subscription auth surfaces no usage events). Cost-based stop is INERT; the cycle cap (%d) governs. Use --cycles N for a deterministic bound on this driver/auth.\n",
						cfg.BudgetUSD, ranCycle, cfg.MaxCycles)
				}
			}
			if cfg.BatchCapUSD > 0 && !checkpointDisabled {
				pct := (lr.TotalCost / cfg.BatchCapUSD) * 100
				if pct >= float64(criticalPct) && cycleEnv["EVOLVE_CHECKPOINT_REQUEST"] != "1" {
					fmt.Fprintf(stderr, "[loop] BATCH-BUDGET CRITICAL: cumulative $%.2f (%.0f%%) >= %d%% — signaling next cycle to checkpoint at phase boundary\n",
						lr.TotalCost, pct, criticalPct)
					cycleEnv["EVOLVE_CHECKPOINT_REQUEST"] = "1"
					cycleEnv["EVOLVE_CHECKPOINT_REASON"] = "batch-cap-near"
				} else if pct >= float64(warnPct) {
					fmt.Fprintf(stderr, "[loop] BATCH-BUDGET WARN: cumulative $%.2f (%.0f%%) >= %d%% — consider operator review\n",
						lr.TotalCost, pct, warnPct)
				}
			}
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

		// Gap #2: rc=4 emission on batch_cap overrun (cycles-mode only).
		// In budget mode hitting cap IS the success signal (handled
		// below). In cycles-mode it's a backstop the operator must
		// raise explicitly. Skipped under BATCH_BUDGET_DISABLE.
		if !batchBudgetDisabled && cfg.BatchCapUSD > 0 && lr.TotalCost > cfg.BatchCapUSD {
			if cfg.BudgetDriven {
				// Gap #4: budget-driven success.
				fmt.Fprintf(stderr, "[loop] BUDGET-EXHAUSTED: cumulative $%.2f >= budget $%.2f (after cycle %d)\n",
					lr.TotalCost, cfg.BudgetUSD, ranCycle)
				lr.StopReason = "budget"
				lr.emit(stdout)
				return 0
			}
			fmt.Fprintf(stderr, "[loop] BATCH-BUDGET-EXCEEDED: cumulative $%.2f > cap $%.2f (after cycle %d)\n",
				lr.TotalCost, cfg.BatchCapUSD, ranCycle)
			fmt.Fprintln(stderr, "[loop]   override: EVOLVE_BATCH_BUDGET_DISABLE=1 or --batch-cap-usd <higher>")
			lr.StopReason = "batch_cap"
			lr.emitFatal(stdout, stderr, cfg, ranCycle)
			return 4
		}

		// Gap #4: budget-driven mode stop condition. In budget mode,
		// loop stops when cumulative cost meets/exceeds the requested
		// budget (success signal). cycles becomes a safety cap only.
		if cfg.BudgetDriven && lr.TotalCost >= cfg.BudgetUSD {
			fmt.Fprintf(stderr, "[loop] BUDGET-DRIVEN COMPLETE: cumulative $%.2f >= budget $%.2f (after cycle %d)\n",
				lr.TotalCost, cfg.BudgetUSD, ranCycle)
			lr.StopReason = "budget"
			lr.emit(stdout)
			return 0
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
		if policy != dispatchPolicyOff {
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

				if policy == dispatchPolicyStop {
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

		if result.FinalVerdict == core.VerdictFAIL {
			lr.StopReason = "fail"
			break
		}
	}

	if lr.StopReason == "error" || lr.StopReason == "fail" {
		lr.emitFatal(stdout, stderr, cfg, lastCycleIn(lr))
		return 2
	}
	lr.emit(stdout)
	// E1 exit-code contract: when any cycle in the batch hit a
	// recoverable failure (verify-fail + classify → recoverable),
	// signal rc=3 so CI sees "batch completed but with infra/audit/
	// build issues". Matches bash dispatcher's DISPATCH_RC=3.
	if lr.RecoverableFailures > 0 {
		return 3
	}
	return 0
}

// wireOrchestratorDepsFn is the test seam for runLoop. Tests
// substitute a stub that returns a fake orchestrator + in-memory
// storage/ledger so the M4 pipeline can be exercised end-to-end
// without spawning real LLM subagents.
var wireOrchestratorDepsFn = wireOrchestratorDeps

// dispatchPolicy enumerates EVOLVE_DISPATCH_POLICY values.
type dispatchPolicy int

const (
	dispatchPolicyVerify dispatchPolicy = iota // default — verify + continue on recoverable, STOP on breach
	dispatchPolicyOff                          // skip ledger pipeline verification entirely (LEGACY)
	dispatchPolicyStop                         // verify + STOP on any failure (legacy fail-fast)
)

const (
	defaultCircuitBreakerThreshold = 5
	defaultCheckpointWarnPct       = 80
	defaultCheckpointCriticalPct   = 95
)

// resolveCheckpointThresholds reads EVOLVE_CHECKPOINT_WARN_AT_PCT and
// EVOLVE_CHECKPOINT_AT_PCT, defaulting to 80 and 95 respectively.
// Invalid / out-of-range values (≤0, >100, or warn ≥ critical) fall
// back to defaults — mirrors the bash dispatcher's lenient parsing at
// lines 1057-1075.
func resolveCheckpointThresholds() (warn, critical int) {
	warn = parsePctEnv("EVOLVE_CHECKPOINT_WARN_AT_PCT", defaultCheckpointWarnPct)
	critical = parsePctEnv("EVOLVE_CHECKPOINT_AT_PCT", defaultCheckpointCriticalPct)
	// Sanity: warn must be below critical, otherwise neither fires
	// meaningfully. Snap back to defaults if operator inverted them.
	if warn <= 0 || critical <= 0 || warn >= critical || critical > 100 {
		return defaultCheckpointWarnPct, defaultCheckpointCriticalPct
	}
	return warn, critical
}

// parsePctEnv reads an int env var, clamping to [1,100]. Out-of-range
// or unparseable values return the supplied default.
func parsePctEnv(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 100 {
		return def
	}
	return n
}

// parseIntEnv reads a positive integer env var. Unparseable / ≤0 values
// fall back to the default. Used by --budget-mode safety-cap resolution.
func parseIntEnv(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// resolveDispatchPolicy reads EVOLVE_DISPATCH_POLICY and bridges the
// deprecated EVOLVE_DISPATCH_VERIFY / EVOLVE_DISPATCH_STOP_ON_FAIL
// flags, mirroring the bash precedence at
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:1130-1148.
//
// Precedence: EVOLVE_DISPATCH_POLICY wins. If unset, STOP_ON_FAIL=1
// maps to dispatchPolicyStop and VERIFY=0 maps to dispatchPolicyOff
// (STOP_ON_FAIL wins on conflict because it's the more restrictive).
func resolveDispatchPolicy(stderr io.Writer) dispatchPolicy {
	if p := os.Getenv("EVOLVE_DISPATCH_POLICY"); p != "" {
		switch p {
		case "off":
			return dispatchPolicyOff
		case "stop":
			return dispatchPolicyStop
		case "verify":
			return dispatchPolicyVerify
		default:
			fmt.Fprintf(stderr, "[loop] WARN: unknown EVOLVE_DISPATCH_POLICY=%q — defaulting to verify\n", p)
			return dispatchPolicyVerify
		}
	}
	legacyStop := os.Getenv("EVOLVE_DISPATCH_STOP_ON_FAIL") == "1"
	legacyVerify := os.Getenv("EVOLVE_DISPATCH_VERIFY") == "0"
	if legacyStop {
		fmt.Fprintln(stderr, "[loop] WARN: EVOLVE_DISPATCH_STOP_ON_FAIL is deprecated; use EVOLVE_DISPATCH_POLICY=stop")
		return dispatchPolicyStop
	}
	if legacyVerify {
		fmt.Fprintln(stderr, "[loop] WARN: EVOLVE_DISPATCH_VERIFY=0 is deprecated; use EVOLVE_DISPATCH_POLICY=off")
		return dispatchPolicyOff
	}
	return dispatchPolicyVerify
}

// resolveCircuitBreakerThreshold reads EVOLVE_DISPATCH_REPEAT_THRESHOLD
// (default 5). Values <= 0 fall back to the default — preventing an
// accidentally-zero env var from instantly tripping the breaker.
func resolveCircuitBreakerThreshold() int {
	v := os.Getenv("EVOLVE_DISPATCH_REPEAT_THRESHOLD")
	if v == "" {
		return defaultCircuitBreakerThreshold
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultCircuitBreakerThreshold
	}
	return n
}

// readLastCycleNumber returns state.LastCycleNumber, or 0 on any error
// (missing state.json is fine — first cycle starts at 1).
func readLastCycleNumber(ctx context.Context, st core.Storage) (int, error) {
	state, err := st.ReadState(ctx)
	if err != nil {
		return 0, err
	}
	return state.LastCycleNumber, nil
}

// unfinishedCycle reports whether cycle-state describes a cycle that started
// but never completed — its id is ahead of lastCycleNumber. A clean cycle
// advances lastCycleNumber to its own id on completion (orchestrator), so a
// finished cycle has cs.CycleID == lastCycleNumber and is NOT flagged; a fresh
// tree has cs.CycleID == 0. Only a genuinely stuck cycle (id > last) trips it.
func unfinishedCycle(cs core.CycleState, lastCycleNumber int) bool {
	return cs.CycleID != 0 && cs.CycleID > lastCycleNumber
}

// cycleWorkspace returns .evolve/runs/cycle-<N>/ for verify/classify.
// Path matches the bash dispatcher's RUNS_DIR + cycle-state.json
// WorkspacePath construction.
func cycleWorkspace(projectRoot string, cycle int) string {
	return filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
}

// updateBreaker is the pure step function of the same-cycle circuit
// breaker. Returns the new (prev, streak, tripped) tuple given the
// current ran_cycle.
//
// Algorithm (port of archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:1110-1128):
//
//	if ranCycle == prev: streak++
//	else: prev = ranCycle, streak = 1
//	tripped = streak >= threshold
//
// Extracted from runLoop so the algorithm is unit-testable without
// gaming the orchestrator's LastCycleNumber bookkeeping.
func updateBreaker(prev, streak, ranCycle, threshold int) (newPrev, newStreak int, tripped bool) {
	if ranCycle == prev {
		streak++
	} else {
		streak = 1
		prev = ranCycle
	}
	return prev, streak, streak >= threshold
}

// budgetGatingUnobservable reports whether cost-based budget gating cannot
// function this run: a --budget-usd run whose completed cycle contributed $0
// cost. That is the signature of an unobservable-cost driver/auth (tmux-driver
// scrollback or subscription auth surfaces no stream-json usage events), so the
// `--budget-usd` stop can never trip and the run would silently fall through to
// the cycle cap while the operator believes spend is bounding it. The loop uses
// this to warn ONCE; it deliberately does NOT fabricate a stop, because true
// cost is unknown — the cap governs and the operator is told to use --cycles N.
func budgetGatingUnobservable(budgetDriven bool, cycleCostDelta float64) bool {
	// Exact == 0 (not an epsilon) is intentional: an unobservable driver/auth
	// produces exactly 0.0 (no usage events written), whereas any sub-cent but
	// non-zero cost means usage IS observable and the budget can gate — don't warn.
	return budgetDriven && cycleCostDelta == 0
}

// quotaPause is the parsed cycle-state.json checkpoint block when the
// dispatcher detects a Claude Code subscription quota wall.
type quotaPause struct {
	Cycle       int
	WakeAt      string
	Source      string
	Attempts    int
	MaxAttempts int
}

// detectQuotaPause reads <evolveDir>/cycle-state.json and returns a
// populated quotaPause when checkpoint.enabled==true AND
// checkpoint.reason=="quota-likely". The bash dispatcher's analog at
// lines 907-930 uses jq; the Go side uses map[string]any for the same
// schema-flexible read.
//
// Returns (zero, false) on any failure path (missing file, malformed
// JSON, wrong reason, or checkpoint disabled) — quota-pause is an
// opt-in signal, not an error condition.
func detectQuotaPause(evolveDir string) (quotaPause, bool) {
	path := filepath.Join(evolveDir, "cycle-state.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return quotaPause{}, false
	}
	var blob map[string]any
	if err := json.Unmarshal(raw, &blob); err != nil {
		return quotaPause{}, false
	}
	cp, ok := blob["checkpoint"].(map[string]any)
	if !ok {
		return quotaPause{}, false
	}
	enabled, _ := cp["enabled"].(bool)
	if !enabled {
		return quotaPause{}, false
	}
	reason, _ := cp["reason"].(string)
	if reason != "quota-likely" {
		return quotaPause{}, false
	}
	qp := quotaPause{
		MaxAttempts: 3, // default per bash dispatcher (autoResumeMaxAttempts // 3)
	}
	// cycle_id has float64 dynamic type from JSON. Fall back to
	// blob["cycle"] (top-level) if cycle_id absent.
	if v, ok := blob["cycle_id"].(float64); ok {
		qp.Cycle = int(v)
	} else if v, ok := blob["cycle"].(float64); ok {
		qp.Cycle = int(v)
	}
	if v, ok := cp["quotaResetAt"].(string); ok {
		qp.WakeAt = v
	}
	if v, ok := cp["quotaResetSource"].(string); ok {
		qp.Source = v
	} else {
		qp.Source = "unknown"
	}
	if v, ok := cp["autoResumeAttempts"].(float64); ok {
		qp.Attempts = int(v)
	}
	if v, ok := cp["autoResumeMaxAttempts"].(float64); ok {
		qp.MaxAttempts = int(v)
	}
	return qp, true
}

// dirExists is a tiny helper for the best-effort emit path. Returns
// true only when the path resolves to an existing directory; broken
// symlinks or files of the same name return false.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// parseLoopArgs parses `evolve loop` arguments per the v11.5.0 M1 CLI
// surface. Returns the resolved config + rc (0 = success, 10 = bad
// args, exits printed to stderr).
//
// Argument precedence:
//
//	--goal-hash takes priority over --goal-text (--goal-text computes hash)
//	--goal-text takes priority over positional [GOAL...]
//	--cycles / --max-cycles take priority over positional [CYCLES]
//	--strategy takes priority over positional [STRATEGY]
//
// Positional parsing matches the bash dispatcher heuristic at
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:325-349:
//
//	first numeric token (if any) → CYCLES
//	next token if matching strategy whitelist → STRATEGY
//	remaining tokens (joined by space) → GOAL
func parseLoopArgs(args []string, stderr io.Writer) (loopConfig, int) {
	fs := flag.NewFlagSet("evolve loop", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		projectRoot    string
		evolveDir      string
		goalHash       string
		goalText       string
		strategy       string
		maxCyclesFlag  int
		cyclesFlag     int
		budgetUSD      float64
		batchCapUSD    float64
		resume         bool
		dryRun         bool
		reset          bool
		consensusAudit bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ (default <project-root>/.evolve)")
	fs.StringVar(&goalHash, "goal-hash", "", "explicit 64-char (or 8-char prefix) SHA256 of goal; mutually exclusive with --goal-text")
	fs.StringVar(&goalText, "goal-text", "", "goal text; hashed via goalhash.Compute (normalize+SHA256)")
	fs.StringVar(&strategy, "strategy", "", "balanced|innovate|harden|repair|ultrathink|autoresearch (default: balanced)")
	fs.IntVar(&maxCyclesFlag, "max-cycles", 0, "maximum cycles to run (default 1; aliased by --cycles)")
	fs.IntVar(&cyclesFlag, "cycles", 0, "alias for --max-cycles")
	fs.Float64Var(&budgetUSD, "budget-usd", 0, "budget-driven mode dollar cap (alias: --budget); stops loop at this cumulative cost with rc=0")
	// --budget is the bash-dispatcher alias for --budget-usd (v8.60.0+).
	// Pass-through to the same variable; whichever is set last wins.
	fs.Float64Var(&budgetUSD, "budget", 0, "alias for --budget-usd")
	fs.Float64Var(&batchCapUSD, "batch-cap-usd", 20.0, "cumulative batch USD cap (trips with rc=4 in cycles-mode)")
	fs.BoolVar(&resume, "resume", false, "locate and resume most-recent checkpointed cycle (protocol lands in M3)")
	fs.BoolVar(&dryRun, "dry-run", false, "parse args, print resolved config as JSON, exit 0 (no orchestrator invocation)")
	fs.BoolVar(&reset, "reset", false, "prune infrastructure-systemic/transient + ship-gate-config from state.json:failedApproaches before loop")
	fs.BoolVar(&consensusAudit, "consensus-audit", false, "opt-in cross-CLI auditor consensus mode")

	// WS-G2 repeatable per-agent overrides:
	//   --cli  auditor=claude-tmux              (one --cli per agent)
	//   --cli  builder=ollama-tmux              (repeatable)
	//   --model auditor=opus
	//   --model builder=llama3.1:8b
	// Syntactic sugar over EVOLVE_<AGENT>_CLI / EVOLVE_<AGENT>_MODEL —
	// operators can experiment with combos per-run without editing profiles.
	perAgentCLI := map[string]string{}
	perAgentModel := map[string]string{}
	fs.Func("cli", "per-agent CLI override (repeatable): --cli auditor=claude-tmux", func(v string) error {
		agent, value, ok := strings.Cut(v, "=")
		if !ok || strings.TrimSpace(agent) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("--cli expects agent=cli (e.g. --cli auditor=claude-tmux); got %q", v)
		}
		perAgentCLI[strings.TrimSpace(agent)] = strings.TrimSpace(value)
		return nil
	})
	fs.Func("model", "per-agent model override (repeatable): --model auditor=opus", func(v string) error {
		agent, value, ok := strings.Cut(v, "=")
		if !ok || strings.TrimSpace(agent) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("--model expects agent=model (e.g. --model auditor=opus); got %q", v)
		}
		perAgentModel[strings.TrimSpace(agent)] = strings.TrimSpace(value)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		return loopConfig{}, 10
	}

	// Enforce the flag's "absolute path" contract for the project root AND the
	// evolve dir. Downstream, WorkspacePath (= <root>/.evolve/runs/cycle-N) and
	// every per-phase artifact path are derived by joining these; worktree phases
	// run the agent with cwd=worktree, so a RELATIVE base makes the agent resolve
	// the artifact path into the worktree subtree while the in-process bridge
	// polls it against the main cwd — that divergence caused cycle-119's
	// ExitArtifactTimeout (81). Resolving once here (the composition root) keeps
	// every derived path cwd-independent. filepath.Abs only errors when os.Getwd
	// fails (cwd deleted/unmounted), in which case continuing with a relative
	// base would silently reproduce the very timeout this guards against — so we
	// WARN loudly rather than swallow it (the loop may still serve non-worktree
	// phases, so we degrade rather than abort).
	absOrWarn := func(label, p string) string {
		return paths.AbsoluteRoot(label, p, func(m string) {
			fmt.Fprintf(stderr, "evolve loop: WARN: %s\n", m)
		})
	}
	projectRoot = absOrWarn("--project-root", projectRoot)

	// --budget-usd / --budget numeric validation (Go flag package allows
	// negative floats; bash dispatcher rejects them).
	if budgetUSD < 0 {
		fmt.Fprintf(stderr, "evolve loop: --budget-usd must be a positive number (got: %g)\n", budgetUSD)
		return loopConfig{}, 10
	}

	// Parse positional args: [CYCLES] [STRATEGY] [GOAL...]
	posCycles, posStrategy, posGoal := parsePositional(fs.Args())

	// Gap #7: legacy positional-integer deprecation WARN. Mirrors the
	// bash dispatcher's v8.60.0+ message — operators relying on bare
	// `/evolve-loop 3 ...` get nudged toward `--cycles 3` / `--budget-usd N`.
	if posCycles > 0 && cyclesFlag == 0 && maxCyclesFlag == 0 && budgetUSD == 0 {
		fmt.Fprintf(stderr, "evolve loop: WARN: bare positional integer (%d) parsed as --cycles; prefer explicit --cycles N or --budget-usd N (deprecated since v8.60.0)\n", posCycles)
	}

	// Resolve cycles: explicit flag > positional > default
	// Default depends on mode: cycles-mode = 1, budget-mode safety cap = 50
	// (or EVOLVE_BUDGET_MAX_CYCLES override).
	budgetMode := budgetUSD > 0
	defaultCycles := 1
	if budgetMode {
		defaultCycles = parseIntEnv("EVOLVE_BUDGET_MAX_CYCLES", 50)
	}
	resolvedCycles := 0
	switch {
	case cyclesFlag > 0:
		resolvedCycles = cyclesFlag
	case maxCyclesFlag > 0:
		resolvedCycles = maxCyclesFlag
	case posCycles > 0:
		resolvedCycles = posCycles
	default:
		resolvedCycles = defaultCycles
	}

	// Resolve strategy: explicit flag > positional > default
	resolvedStrategy := strategy
	if resolvedStrategy == "" {
		resolvedStrategy = posStrategy
	}
	if resolvedStrategy == "" {
		resolvedStrategy = "balanced"
	}
	if _, ok := validStrategies[resolvedStrategy]; !ok {
		fmt.Fprintf(stderr, "evolve loop: invalid --strategy %q (valid: balanced|innovate|harden|repair|ultrathink|autoresearch)\n", resolvedStrategy)
		return loopConfig{}, 10
	}

	// Resolve goal: --goal-hash > --goal-text > positional [GOAL...]
	resolvedGoalText := goalText
	if resolvedGoalText == "" && posGoal != "" {
		resolvedGoalText = posGoal
	}
	resolvedGoalHash := goalHash
	if resolvedGoalHash == "" && resolvedGoalText != "" {
		resolvedGoalHash = goalhash.Compute(resolvedGoalText)
	}
	// Resume mode is the one path that doesn't require an explicit goal —
	// the resume protocol reads goal from cycle-state.json.
	if resolvedGoalHash == "" && !resume {
		fmt.Fprintln(stderr, "evolve loop: a goal is required — pass --goal-hash, --goal-text, or a positional goal (or --resume to continue a checkpointed cycle)")
		return loopConfig{}, 10
	}

	// Resolve budget. budgetMode (computed above) is the source of
	// truth — anything >0 is budget-driven. When unset, sentinel
	// 999999 is used internally as "no cap" so the loop runs
	// MaxCycles cycles regardless of cost.
	resolvedBudget := budgetUSD
	if resolvedBudget == 0 {
		resolvedBudget = 999999
	}
	budgetDriven := budgetMode

	// Resolve evolve-dir. The derived branch inherits projectRoot's (now
	// absolute) anchor; an explicit --evolve-dir may still be relative, so
	// absolutize the final value either way (same cwd-independence requirement
	// as projectRoot — many consumers join cfg.EvolveDir).
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}
	evolveDir = absOrWarn("--evolve-dir", evolveDir)

	return loopConfig{
		ProjectRoot:    projectRoot,
		EvolveDir:      evolveDir,
		GoalHash:       resolvedGoalHash,
		GoalText:       resolvedGoalText,
		Strategy:       resolvedStrategy,
		MaxCycles:      resolvedCycles,
		BudgetUSD:      resolvedBudget,
		BatchCapUSD:    batchCapUSD,
		Resume:         resume,
		Reset:          reset,
		ConsensusAudit: consensusAudit,
		DryRun:         dryRun,
		BudgetDriven:   budgetDriven,
		PerAgentCLI:    perAgentCLI,
		PerAgentModel:  perAgentModel,
	}, 0
}

// parsePositional consumes the [CYCLES] [STRATEGY] [GOAL...] positional
// args per the bash dispatcher's heuristic.
//
//	First token is CYCLES iff it's a positive integer.
//	Next token is STRATEGY iff it's in validStrategies.
//	Remaining tokens are joined by space → GOAL.
//
// Order matters; this matches the bash heuristic verbatim so operators
// who paste their `/evolve-loop 3 balanced "fix bug"` invocations into
// the Go binary keep the same parsing semantics.
func parsePositional(args []string) (cycles int, strategy string, goal string) {
	i := 0
	if i < len(args) {
		if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
			cycles = n
			i++
		}
	}
	if i < len(args) {
		if _, ok := validStrategies[args[i]]; ok {
			strategy = args[i]
			i++
		}
	}
	if i < len(args) {
		goal = joinArgs(args[i:])
	}
	return
}

// joinArgs joins args with a single space. Empty slice → empty string.
// Preserves inner quoting the way bash does when the operator quotes
// a multi-word goal in the original CLI invocation.
func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

// buildCycleContext returns the Context map handed to every cycle.
// Phase agents read it via PhaseRequest.Context: Scout for strategy,
// Intent for the canonical goal text (used to structure intent.md
// before Scout sees it).
//
// Pre-this-fix, only "strategy" was passed — `cfg.GoalText` was
// converted to a hash at parse time and the text discarded. Intent
// persona had no way to see the operator's goal, so intent.md was
// being structured around whatever leftover Scout artifacts happened
// to be in the workspace. Source incident: cycle-108 meta-loop where
// the user's "non-stop autonomy + /goal comparison" goal-text was
// dropped and intent.md got structured around the prior cycle's
// untested-package backlog work instead.
func buildCycleContext(cfg loopConfig) map[string]string {
	out := map[string]string{
		"strategy": cfg.Strategy,
	}
	if cfg.GoalText != "" {
		out["goal"] = cfg.GoalText
	}
	return out
}

// buildCycleEnv returns the env map handed to every cycle in this
// dispatcher invocation. Construction order is intentional:
//
//  1. Copy every EVOLVE_* var from osEnv. This is how operator-set
//     flags (REQUIRE_INTENT, SANDBOX_FALLBACK_ON_EPERM, TRIAGE_DISABLE,
//     BUILD_PLANNER, STDOUT_FILTER, …) reach the orchestrator + every
//     downstream subagent.
//  2. Apply dispatcher-derived overrides (Strategy, ConsensusAudit,
//     Resume, Reset). CLI-derived choices win over env so an operator
//     passing `--strategy harden` overrides EVOLVE_STRATEGY=balanced.
//
// Non-EVOLVE_* vars are intentionally skipped — only this prefix is
// part of the documented operator surface. The orchestrator reads from
// the returned map, never from os.Environ directly, so callers that
// inject env explicitly (tests, in-process embedders) get the same path.
func buildCycleEnv(cfg loopConfig, osEnv []string) map[string]string {
	out := make(map[string]string, 16)
	for _, kv := range osEnv {
		if !strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		out[kv[:eq]] = kv[eq+1:]
	}
	// Dispatcher-derived overrides — CLI > env.
	out["EVOLVE_STRATEGY"] = cfg.Strategy
	if cfg.ConsensusAudit {
		out["EVOLVE_CONSENSUS_AUDIT"] = "1"
	}
	if cfg.Resume {
		out["EVOLVE_RESUME"] = "1"
	}
	if cfg.Reset {
		out["EVOLVE_RESET"] = "1"
	}
	// WS-G2: per-agent --cli / --model launch flags translate to
	// EVOLVE_<AGENT>_CLI / EVOLVE_<AGENT>_MODEL env keys (matching
	// envchain.PhaseEnvKey's convention). The runner already reads these
	// for the CLI resolver (G1) and the model resolver. Flag overrides win
	// over inherited process env (their entries are written after the
	// EVOLVE_* sweep above).
	for agent, cli := range cfg.PerAgentCLI {
		out["EVOLVE_"+phaseEnvAgentKey(agent)+"_CLI"] = cli
	}
	for agent, model := range cfg.PerAgentModel {
		out["EVOLVE_"+phaseEnvAgentKey(agent)+"_MODEL"] = model
	}
	return out
}

// phaseEnvAgentKey upper-cases + dash-to-underscore an agent name to
// build per-agent env keys (mirror of envchain.PhaseEnvKey's normalization).
// e.g. "tdd-engineer" → "TDD_ENGINEER" so EVOLVE_TDD_ENGINEER_CLI/MODEL
// match the runner's lookup.
func phaseEnvAgentKey(agent string) string {
	b := make([]byte, 0, len(agent))
	for i := 0; i < len(agent); i++ {
		c := agent[i]
		switch {
		case c == '-':
			b = append(b, '_')
		case c >= 'a' && c <= 'z':
			b = append(b, c-32)
		default:
			b = append(b, c)
		}
	}
	return string(b)
}
