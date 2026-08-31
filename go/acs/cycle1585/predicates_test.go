//go:build acs

// Package cycle1585 materializes the cycle-1585 acceptance criteria for the sole
// committed task of this fleet lane, quota-defer-short-circuits-retro
// (scout-report.md ## Selected Tasks; triage-decision.json ## top_n). Per R9.3
// no predicate binds to the lane's deferred item (quota-reset-evidence-go-producer).
//
// The defect: the all-families-quota-exhausted abort (cyclerun_dispatch.go:264-287)
// is a DEFERRED, resumable outcome — checkpoint written, typed
// ErrAllFamiliesExhausted returned, loop exits rc=5 — yet it still calls
// cr.recordFailureLearning, whose guard (failure_learning.go:344) short-circuits
// only for fl.Failed == PhaseRetro. With no clause for the sentinel the DEFERRED
// path mutates CycleState to "retro" and runs a whole retro phase against the
// quota wall that just drained every family.
//
// AC map (1:1 with scout-report.md ## Selected Tasks "Acceptance" + "verifiableBy"):
//
//	AC1 "an all-families-quota-exhausted dispatch never calls the retro runner
//	     (counting-fake asserts 0 calls)"
//	    → C1585_001 (runs the production-path counting test in internal/core)
//	AC2 "CycleState.Phase/ActiveAgent are never set to retro for this path"
//	    → C1585_002
//	AC3 "deterministic state.FailedAt bookkeeping is still recorded" + the
//	     anti-gaming edge (multiply-wrapped sentinel matched via errors.Is)
//	    → C1585_003
//	AC4 "a genuine (non-quota) failure still dispatches retro exactly once —
//	     no regression"  → C1585_004 (NEGATIVE; pre-existing GREEN, bound so a
//	     fix that short-circuits unconditionally cannot pass this cycle)
//	AC5 "go test -count=1 ./internal/core/... exits 0 overall"
//	    → manual+checklist in test-report.md (a whole-package sweep of
//	      ./internal/core is a banned flaky predicate shape under fleet load;
//	      the CI go job and the ship gate already run it)
//	AC6 "one materialized eval with code-graded checks"  → C1585_005
//
// Adversarial axes: negative (C1585_004 — the fix must NOT swallow ordinary
// failures; C1585_005 rejects a vacuous zero-command eval), edge (C1585_003 —
// the sentinel arrives multiply %w-wrapped, so an `==` identity check fails
// here), semantic (zero-dispatch, resumable cycle-state, bookkeeping survival
// and eval rigor are four distinct behaviors, not one restated).
//
// No source-grep predicates (cycle-85 rule): C1585_001..004 execute the real
// production dispatch chain as a subprocess and count named PASS markers (a
// bare exit 0 would hide a renamed or skipped test); C1585_005 runs the SSOT
// eval-quality checker. Every `go test` invocation names ONE package and is
// narrowed with -run (flaky-predicate-shape rule: no `/...` sweeps, no
// unnarrowed ./internal/core).
package cycle1585

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	evalSlug = "quota-defer-short-circuits-retro"
)

// runCoreTest runs exactly one named test in internal/core and returns its
// verbose output. One named package, always -run-narrowed — never a /... sweep.
func runCoreTest(t *testing.T, name string) string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+name+"$", corePkg)
	if code != 0 || err != nil {
		t.Errorf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, corePkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("%s did not report PASS (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
	}
	return stdout
}

// AC1: the production RunCycle chain drives scout to exit=85 on every attempt
// and the counting fake must observe ZERO retro-runner calls. A green unit test
// on the guard helper alone already passed once in cycle-1582 while the wiring
// was still broken, so the predicate binds the whole dispatch chain.
func TestC1585_001_all_families_exhausted_dispatches_no_retro(t *testing.T) {
	runCoreTest(t, "TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro")
}

// AC2: no cycle-state the orchestrator PERSISTS on that path may say "retro" —
// in Phase, ActiveAgent, or CompletedPhases. A checkpoint that names retro makes
// `evolve loop --resume` re-enter retro instead of the exhausted phase, so
// skipping only the runner call is not sufficient.
func TestC1585_002_deferred_cycle_state_never_says_retro(t *testing.T) {
	runCoreTest(t, "TestRunCycle_AllFamiliesExhausted_NeverWritesRetroCycleState")
}

// AC3 (edge / anti-gaming): the sentinel reaches recordFailureLearning wrapped
// at least twice. Driving the chokepoint directly with a deliberately
// over-wrapped error makes a cheap `fl.Err == ErrAllFamiliesExhausted` identity
// check fail, and the same test pins that state.FailedAt is still recorded —
// the short-circuit must land AFTER recordFailedApproachState, not replace it.
func TestC1585_003_wrapped_sentinel_matched_and_bookkeeping_preserved(t *testing.T) {
	runCoreTest(t, "TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro")
}

// AC4 (NEGATIVE): an ordinary non-quota dispatch failure is a real FAIL and must
// still reach retro exactly once. A guard that short-circuits every failure — or
// keys off "any transient error" instead of the typed sentinel — passes AC1 and
// dies here. This is the anti-no-op predicate for this cycle.
func TestC1585_004_non_quota_failure_still_dispatches_retro_once(t *testing.T) {
	runCoreTest(t, "TestRunCycle_NonQuotaDispatchFailure_StillDispatchesRetroOnce")
}

// AC6: the task's eval file passes the SSOT quality checker (the exact code
// behind `evolve eval quality-check`) over a NON-EMPTY command set — an eval
// with no fenced command block PASSes vacuously, which is the existence-check
// gaming the contract forbids.
func TestC1585_005_eval_file_passes_quality_check(t *testing.T) {
	evalPath := filepath.Join(acsassert.RepoRoot(t), ".evolve", "evals", evalSlug+".md")
	res, err := evalqualitycheck.Check(evalqualitycheck.Options{Path: evalPath})
	if err != nil {
		t.Fatalf("eval quality-check %s: %v", evalPath, err)
	}
	if res.Overall != evalqualitycheck.LevelPass {
		for _, c := range res.Commands {
			if c.Level != evalqualitycheck.LevelPass {
				t.Errorf("eval command %q classified level %d: %s", c.Line, c.Level, c.Reason)
			}
		}
		t.Fatalf("eval %s overall level %d, want PASS(0)", evalPath, res.Overall)
	}
	if len(res.Commands) < 2 {
		t.Fatalf("eval %s classified only %d command(s) — a vacuous eval is not a PASS",
			evalPath, len(res.Commands))
	}
}
