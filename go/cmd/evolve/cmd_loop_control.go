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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

var wireOrchestratorDepsFn = wireOrchestratorDeps

// disableWorkspaceGuardForTest is a test seam: package-level test harnesses
// that pre-seed cycle workspaces (M4/M5 dispatch validators, etc.) set this
// to true so the orchestrator does not archive the pre-seeded files before
// phases run. Production code always leaves this false. Replaces the retired
// EVOLVE_DISABLE_WORKSPACE_GUARD env signal (cycle-10 flag-reduction).
var disableWorkspaceGuardForTest bool

// dispatchPolicy enumerates the dispatch verification policy values.
type dispatchPolicy int

const (
	dispatchPolicyVerify dispatchPolicy = iota // default — verify + continue on recoverable, STOP on breach
	dispatchPolicyOff                          // skip ledger pipeline verification entirely
	dispatchPolicyStop                         // verify + STOP on any failure (legacy fail-fast)
)

const defaultCircuitBreakerThreshold = 5

// orphanGCTimeout bounds the crash-recovery session sweep so a wedged tmux
// socket (corrupted server, not the common "no server" case) can never hang the
// loop — the GC must stay robust even when the surrounding pipeline is broken.
const orphanGCTimeout = 15 * time.Second

// resolveDispatchPolicy maps a policy string (from DispatchConfig.Policy) to
// the corresponding dispatch policy. Unknown values default to
// dispatchPolicyVerify with a WARN logged to stderr.
func resolveDispatchPolicy(policyVal string, stderr io.Writer) dispatchPolicy {
	if policyVal != "" {
		switch policyVal {
		case "off":
			return dispatchPolicyOff
		case "stop":
			return dispatchPolicyStop
		case "verify":
			return dispatchPolicyVerify
		default:
			fmt.Fprintf(stderr, "[loop] WARN: unknown dispatch policy %q — defaulting to verify\n", policyVal)
			return dispatchPolicyVerify
		}
	}
	return dispatchPolicyVerify
}

// resolveCircuitBreakerThreshold maps a RepeatThreshold from DispatchConfig to
// the breaker value. Values <= 0 fall back to the default — preventing an
// accidentally-zero config from instantly tripping the breaker.
func resolveCircuitBreakerThreshold(threshold int) int {
	if threshold <= 0 {
		return defaultCircuitBreakerThreshold
	}
	return threshold
}

// loadDispatchConfig loads .evolve/policy.json and returns the dispatch config
// with defaults resolved. Absent or malformed policy ⇒ built-in defaults.
func loadDispatchConfig(evolveDir string) policy.DispatchConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.DispatchConfig{Policy: "verify", RepeatThreshold: defaultCircuitBreakerThreshold}
	}
	return pol.DispatchConfig()
}

// loadWorkflowConfig loads .evolve/policy.json and returns workflow defaults.
// Absent or malformed policy falls back to built-in defaults.
func loadWorkflowConfig(evolveDir string) policy.WorkflowConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.Policy{}.WorkflowConfig()
	}
	return pol.WorkflowConfig()
}

// loadClassifyConfig loads .evolve/policy.json and returns classifier defaults.
// Absent or malformed policy falls back to built-in defaults (HangClassifier=false).
func loadClassifyConfig(evolveDir string) policy.ClassifyPolicy {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.ClassifyPolicy{}
	}
	return pol.ClassifyConfig()
}

// recordAbsorbedFail persists a continue-on-fail-absorbed verdict-FAIL cycle
// to state.json:failedApproaches. The clean-completion FAIL path (audit
// FAIL → retro → end, err==nil) is NOT recorded by the orchestrator — only
// err!=nil cycle-level failures are — and recordLoopFatal runs only on the
// STOP path the breaker is skipping, so without this the next cycle's scout
// would see no record of the failure. A missing state.json is a soft WARN
// (matching the recoverable-failure path). Best-effort: a record failure
// must not change the loop's continue decision.
func recordAbsorbedFail(cfg loopConfig, ranCycle int, stderr io.Writer) {
	workspace := cycleWorkspace(cfg.ProjectRoot, ranCycle)
	cls := cycleclassify.Classify(workspace)
	_, err := failurelog.Record(
		filepath.Join(cfg.EvolveDir, "state.json"),
		filepath.Join(cfg.EvolveDir, "runs"),
		failurelog.RecordRequest{
			Cycle:          ranCycle,
			Classification: string(cls.Class),
			ReportPath:     filepath.Join(workspace, "orchestrator-report.md"),
			Now:            time.Now().UTC(),
		})
	if err != nil && !errors.Is(err, failurelog.ErrStateMissing) {
		fmt.Fprintf(stderr, "[loop] WARN: could not record absorbed FAIL for cycle %d: %v\n", ranCycle, err)
	}
}

// loopCycleRunner is the slice of *core.Orchestrator the batch loop drives —
// narrowed to an interface so a scripted fake can exercise the loop's
// post-cycle wiring (the consecutive-fail breaker, recordAbsorbedFail) which
// only fires on FinalVerdict=FAIL, a state the real orchestrator cannot reach
// without a full phase machine.
type loopCycleRunner interface {
	RunCycle(context.Context, core.CycleRequest) (core.CycleResult, error)
	RunCycleFromPhase(context.Context, core.CycleRequest, *core.ResumePoint) (core.CycleResult, error)
}

// loopOrchOverride is a test-only seam: when non-nil, runLoop drives this
// instead of the wired *core.Orchestrator. nil in production.
var loopOrchOverride loopCycleRunner

// consecutiveFailBreaker advances the consecutive-verdict-FAIL streak and
// reports whether the batch must stop. A non-FAIL cycle resets the streak to
// zero (a PASS/SHIPPED breaks the run); a FAIL increments it and stops once
// the streak reaches max. WorkflowConfig guarantees max is always ≥1, so
// max==1 stops on the first FAIL — byte-identical to the
// pre-flag unconditional break. Pure for testability, mirroring
// updateBreaker (the same-cycle dispatcher breaker).
func consecutiveFailBreaker(failed bool, streak, max int) (newStreak int, stop bool) {
	if !failed {
		return 0, false
	}
	streak++
	return streak, streak >= max
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

// reapCycleSessions kills any tmux sessions the cycle's launches registered
// in its per-run registry (CB.5: by registry, never glob — see
// swarm.ReapRunSessions). Fired after EVERY cycle attempt: a clean cycle's
// sessions were already killed by per-launch cleanup (re-kill is a no-op);
// an aborted cycle's leaked sessions are exactly what the looppreflight
// stale-session check used to find a batch too late.
func reapCycleSessions(projectRoot string, cycle int, stderr io.Writer) {
	if cycle <= 0 {
		return
	}
	path := sessionrecord.PathIn(cycleWorkspace(projectRoot, cycle))
	rep := swarm.ReapRunSessions(context.Background(), path, swarm.ExecTmuxKill)
	if rep.Killed > 0 || rep.Errors > 0 || rep.Skipped > 0 {
		fmt.Fprintf(stderr, "[loop] session registry reap cycle=%d: killed=%d skipped=%d errors=%d\n",
			cycle, rep.Killed, rep.Skipped, rep.Errors)
	}
	// Belt-and-suspenders: a liveness sweep also reaps any orphan whose creator
	// PID is dead — sessions a crashed phase left behind that never made it into
	// (or were lost from) the registry. Safe under concurrency: live runs' PIDs
	// are alive, so their sessions are skipped.
	gcOrphanSessions(fmt.Sprintf("cycle=%d", cycle), stderr)
}

// gcOrphanSessions runs the crash-recovery liveness GC (swarm.ExecReapOrphans):
// it reaps evolve-namespace tmux sessions whose creator PID is dead — corpses a
// crashed or SIGKILL'd run left on the shared bridge server. It complements the
// per-run registry reap, which is structurally blind to a crashed run's (and any
// other run's) sessions; a SIGKILL'd loop never runs its teardown, so without
// this the next loop inherits the corpses until the server starves the machine.
// Liveness-scoped, so a LIVE concurrent run is never touched. Quiet on a clean
// sweep; logs only when it killed or errored.
func gcOrphanSessions(label string, stderr io.Writer) {
	ctx, cancel := context.WithTimeout(context.Background(), orphanGCTimeout)
	defer cancel()
	rep := swarm.ExecReapOrphans(ctx)
	if len(rep.Killed) > 0 || len(rep.Errors) > 0 {
		fmt.Fprintf(stderr, "[loop] orphan-session GC (%s): killed=%d skipped(live=%d foreign=%d no-pid=%d) errors=%d\n",
			label, len(rep.Killed), rep.SkippedLive, rep.SkippedForeign, rep.SkippedUnparseable, len(rep.Errors))
	}
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
