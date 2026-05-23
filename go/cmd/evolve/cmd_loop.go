// `evolve loop` drives the cycle dispatcher loop with batch budget
// enforcement. Sequential by design — each cycle blocks the next until
// it completes or trips the batch cap (matches v8.34.0+ bash
// dispatcher behavior).
//
// v11.5.0 M1–M6: CLI surface mirrors the now-archived bash dispatcher
// at archive/archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh — positional
// args ([CYCLES] [STRATEGY] [GOAL...]), --goal-text (computes hash via
// goalhash.Compute), --strategy, --resume, --dry-run, --reset,
// --consensus-audit. Existing --goal-hash callers continue to work
// unchanged. EVOLVE_USE_LEGACY_BASH=1 exec's the archived bash path.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/goalhash"
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

// loopConfig is the resolved invocation. Extracted so --dry-run and
// tests can inspect what would be done without invoking the
// orchestrator.
type loopConfig struct {
	ProjectRoot     string  `json:"project_root"`
	EvolveDir       string  `json:"evolve_dir"`
	GoalHash        string  `json:"goal_hash"`
	GoalText        string  `json:"goal_text,omitempty"`
	Strategy        string  `json:"strategy"`
	MaxCycles       int     `json:"max_cycles"`
	BudgetUSD       float64 `json:"budget_usd"`
	BatchCapUSD     float64 `json:"batch_cap_usd"`
	Resume          bool    `json:"resume,omitempty"`
	Reset           bool    `json:"reset,omitempty"`
	ConsensusAudit  bool    `json:"consensus_audit,omitempty"`
	DryRun          bool    `json:"dry_run,omitempty"`
	BudgetDriven    bool    `json:"budget_driven,omitempty"`
}

// runLoop implements `evolve loop`.
func runLoop(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	cfg, rc := parseLoopArgs(args, stderr)
	if rc != 0 {
		return rc
	}

	if cfg.DryRun {
		buf, _ := json.MarshalIndent(map[string]any{
			"dry_run": true,
			"config":  cfg,
		}, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		return 0
	}

	// M6 rollback hatch (v11.5.0+): when EVOLVE_USE_LEGACY_BASH=1, exec
	// the archived bash dispatcher with the original argv. Documented
	// in CLAUDE.md env-var table. The archived path is
	// archive/archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh under the
	// project root. exec semantics replace this process so the parent
	// shell sees the bash exit code directly — same as v11.0–v11.4
	// behavior where this env shelled to the live legacy path.
	if os.Getenv("EVOLVE_USE_LEGACY_BASH") == "1" {
		return execLegacyBash(cfg.ProjectRoot, args, stderr)
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

	// Strategy + bypass-env propagation. Subagents read EVOLVE_STRATEGY
	// to select their prompt variant; Scout also reads Context["strategy"].
	cycleEnv := map[string]string{
		"EVOLVE_STRATEGY": cfg.Strategy,
	}
	if cfg.ConsensusAudit {
		cycleEnv["EVOLVE_CONSENSUS_AUDIT"] = "1"
	}
	if cfg.Resume {
		cycleEnv["EVOLVE_RESUME"] = "1"
	}
	if cfg.Reset {
		cycleEnv["EVOLVE_RESET"] = "1"
	}
	cycleCtx := map[string]string{
		"strategy": cfg.Strategy,
	}

	type loopResult struct {
		StopReason          string             `json:"stop_reason"`
		Cycles              []core.CycleResult `json:"cycles"`
		TotalCost           float64            `json:"total_cost_usd"`
		Resumed             bool               `json:"resumed,omitempty"`
		RecoverableFailures int                `json:"recoverable_failures,omitempty"`
	}
	lr := loopResult{StopReason: "max_cycles"}

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
			buf, _ := json.MarshalIndent(lr, "", "  ")
			fmt.Fprintln(stdout, string(buf))
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
		buf, _ := json.MarshalIndent(lr, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		if lr.StopReason == "error" || lr.StopReason == "fail" {
			return 2
		}
		return 0
	}

	policy := resolveDispatchPolicy(stderr)
	threshold := resolveCircuitBreakerThreshold()

	// Circuit-breaker state. PREV_RAN_CYCLE tracks the cycle number
	// returned by the most-recent RunCycle; SAME_CYCLE_STREAK counts
	// consecutive identical values. Trips at threshold.
	prevRanCycle := -1
	sameCycleStreak := 0

	for i := 0; i < cfg.MaxCycles; i++ {
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
		if cs, err := cyclecost.SummarizeCycle(workspace, ranCycle); err == nil {
			lr.TotalCost += cs.Total.CostUSD
			fmt.Fprintf(stderr, "[loop] cycle %d cost: $%.4f (batch total: $%.4f / cap $%.2f)\n",
				ranCycle, cs.Total.CostUSD, lr.TotalCost, cfg.BatchCapUSD)
		}
		if cfg.BatchCapUSD > 0 {
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
			buf, _ := json.MarshalIndent(lr, "", "  ")
			fmt.Fprintln(stdout, string(buf))
			return 1
		}

		// Verify + classify pipeline (D1 + D2 wired together). Skipped
		// when EVOLVE_DISPATCH_POLICY=off. On verify-fail in `verify`
		// mode, classify + emit event + continue (recoverable classes)
		// or break (integrity-breach). On `stop` mode, any verify-fail
		// halts the batch.
		if policy != dispatchPolicyOff {
			vc := ledgerverify.LoadVerifyContext(workspace, cfg.EvolveDir)
			vr, vErr := ledgerverify.VerifyCycle(context.Background(), deps.Ledger, ranCycle, ledgerverify.Options{
				IntentRequired: vc.IntentRequired,
				CycleVerdict:   vc.CycleVerdict,
			})
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
					buf, _ := json.MarshalIndent(lr, "", "  ")
					fmt.Fprintln(stdout, string(buf))
					return 2
				}
				// policy == verify: STOP only on integrity-breach;
				// recoverable classes continue the loop.
				if class.Class == cycleclassify.ClassIntegrityBreach {
					lr.StopReason = "integrity_breach"
					buf, _ := json.MarshalIndent(lr, "", "  ")
					fmt.Fprintln(stdout, string(buf))
					return 2
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
						buf, _ := json.MarshalIndent(lr, "", "  ")
						fmt.Fprintln(stdout, string(buf))
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

	buf, _ := json.MarshalIndent(lr, "", "  ")
	fmt.Fprintln(stdout, string(buf))
	if lr.StopReason == "error" || lr.StopReason == "fail" {
		return 2
	}
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

// legacyDispatcherPath resolves the archived bash dispatcher under
// projectRoot. Defaults to archive/legacy/scripts/dispatch/
// evolve-loop-dispatch.sh; tests can override via execLegacyBashFn.
func legacyDispatcherPath(projectRoot string) string {
	return filepath.Join(projectRoot, "archive", "legacy", "scripts", "dispatch", "evolve-loop-dispatch.sh")
}

// execLegacyBashFn is the test seam for the rollback hatch. Production
// is execLegacyBashReal which syscall.Exec replaces the current process
// with the bash dispatcher; tests substitute a stub that records the
// call without actually exec'ing.
var execLegacyBashFn = execLegacyBashReal

// execLegacyBash is the rollback hatch — re-runs the original argv
// through the archived bash dispatcher when EVOLVE_USE_LEGACY_BASH=1.
// The Go process is replaced by bash via syscall.Exec so exit codes
// + signal handling stay identical to the v10.x behavior.
func execLegacyBash(projectRoot string, args []string, stderr io.Writer) int {
	return execLegacyBashFn(projectRoot, args, stderr)
}

func execLegacyBashReal(projectRoot string, args []string, stderr io.Writer) int {
	dispatcher := legacyDispatcherPath(projectRoot)
	if _, err := os.Stat(dispatcher); err != nil {
		fmt.Fprintf(stderr, "evolve loop: EVOLVE_USE_LEGACY_BASH=1 but %s not found: %v\n", dispatcher, err)
		return 1
	}
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		fmt.Fprintf(stderr, "evolve loop: EVOLVE_USE_LEGACY_BASH=1 but bash not on PATH: %v\n", err)
		return 1
	}
	// argv0=bash, argv1=dispatcher, argv2... = original loop args.
	bashArgs := append([]string{"bash", dispatcher}, args...)
	fmt.Fprintf(stderr, "[loop] EVOLVE_USE_LEGACY_BASH=1 → exec %s\n", dispatcher)
	// syscall.Exec replaces the current process — only returns on error.
	if err := syscall.Exec(bashPath, bashArgs, os.Environ()); err != nil {
		fmt.Fprintf(stderr, "evolve loop: exec legacy bash: %v\n", err)
		return 1
	}
	// Unreachable on success.
	return 0
}

// dispatchPolicy enumerates EVOLVE_DISPATCH_POLICY values.
type dispatchPolicy int

const (
	dispatchPolicyVerify dispatchPolicy = iota // default — verify + continue on recoverable, STOP on breach
	dispatchPolicyOff                           // skip ledger pipeline verification entirely (LEGACY)
	dispatchPolicyStop                          // verify + STOP on any failure (legacy fail-fast)
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
		projectRoot     string
		evolveDir       string
		goalHash        string
		goalText        string
		strategy        string
		maxCyclesFlag   int
		cyclesFlag      int
		budgetUSD       float64
		batchCapUSD     float64
		resume          bool
		dryRun          bool
		reset           bool
		consensusAudit  bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&evolveDir, "evolve-dir", "", "path to .evolve/ (default <project-root>/.evolve)")
	fs.StringVar(&goalHash, "goal-hash", "", "explicit 64-char (or 8-char prefix) SHA256 of goal; mutually exclusive with --goal-text")
	fs.StringVar(&goalText, "goal-text", "", "goal text; hashed via goalhash.Compute (normalize+SHA256)")
	fs.StringVar(&strategy, "strategy", "", "balanced|innovate|harden|repair|ultrathink|autoresearch (default: balanced)")
	fs.IntVar(&maxCyclesFlag, "max-cycles", 0, "maximum cycles to run (default 1; aliased by --cycles)")
	fs.IntVar(&cyclesFlag, "cycles", 0, "alias for --max-cycles")
	fs.Float64Var(&budgetUSD, "budget-usd", 0, "per-cycle USD budget cap (default 999999)")
	fs.Float64Var(&batchCapUSD, "batch-cap-usd", 20.0, "cumulative batch USD cap (trips with non-zero exit)")
	fs.BoolVar(&resume, "resume", false, "locate and resume most-recent checkpointed cycle (protocol lands in M3)")
	fs.BoolVar(&dryRun, "dry-run", false, "parse args, print resolved config as JSON, exit 0 (no orchestrator invocation)")
	fs.BoolVar(&reset, "reset", false, "prune infrastructure-systemic/transient + ship-gate-config from state.json:failedApproaches before loop")
	fs.BoolVar(&consensusAudit, "consensus-audit", false, "opt-in cross-CLI auditor consensus mode")

	if err := fs.Parse(args); err != nil {
		return loopConfig{}, 10
	}

	// Parse positional args: [CYCLES] [STRATEGY] [GOAL...]
	posCycles, posStrategy, posGoal := parsePositional(fs.Args())

	// Resolve cycles: explicit flag > positional > default
	resolvedCycles := 0
	switch {
	case cyclesFlag > 0:
		resolvedCycles = cyclesFlag
	case maxCyclesFlag > 0:
		resolvedCycles = maxCyclesFlag
	case posCycles > 0:
		resolvedCycles = posCycles
	default:
		resolvedCycles = 1
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

	// Resolve budget: default 999999 (effectively no per-cycle cap).
	resolvedBudget := budgetUSD
	if resolvedBudget == 0 {
		resolvedBudget = 999999
	}
	budgetDriven := budgetUSD > 0 && budgetUSD < 999999

	// Resolve evolve-dir.
	if evolveDir == "" {
		evolveDir = filepath.Join(projectRoot, ".evolve")
	}

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

// joinArgs joins args with a single space, preserving inner quoting
// the way bash would when the operator quoted the goal in the original
// CLI invocation. Empty slice → empty string.
func joinArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	if len(args) == 1 {
		return args[0]
	}
	out := args[0]
	for _, a := range args[1:] {
		out += " " + a
	}
	return out
}
