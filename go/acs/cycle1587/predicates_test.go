//go:build acs

// Package cycle1587 materializes the cycle-1587 acceptance criteria for the
// sole fleet-assigned committed task, pipeline-defect-pipeline-blocker-cycle1582
// (scout-report.md ## Selected Tasks; triage priority P0). The lane's other
// assigned id, tokenopt-session-resume-on-retry, is DEFERRED per R9.3 — no
// predicate binds to it.
//
// The defect (cycle-1585 closed half of it; this cycle closes the rest):
// go/internal/core/cyclerun_dispatch.go's all-CLI-families-quota-exhausted
// abort is a DEFERRED, resumable checkpoint (typed ErrAllFamiliesExhausted,
// loop exits rc=5, `evolve loop --resume` re-enters the drained phase) — not a
// diagnosed phase failure. cycle-1585 stopped the runner from force-dispatching
// retro on that arm, but go/internal/core/failure_learning.go:recordFailureLearning
// still called recordFailedApproachState first, unconditionally appending a
// FailedRecord to state.FailedAt and queuing a P0 "cycle-N-failed-<phase>"
// carryover todo before its ErrAllFamiliesExhausted short-circuit — the
// cycle-1582 dossier root cause: that spurious todo competed with real work on
// every quota wall. The fix moves the short-circuit BEFORE
// recordFailedApproachState and threads a ClassificationOverride so a
// resource-abort at the dispatch boundary can never adopt a stale phase
// self-report (a prior code-failure report left over from an earlier attempt).
//
// AC map (1:1 with scout-report.md ## Selected Tasks "Acceptance Criteria"):
//
//	AC1 "a final negative verdict with a runtime-minted substantive explanation
//	     remains coherent — does not trigger verdict-incoherence"
//	    → C1587_001 (pre-existing GREEN: go/internal/core/system_failure_test.go
//	      already covers the audit- and ship-phase SubstantiveError carriers,
//	      cycles-930/931/932 and cycle-1329; this predicate pins that regression
//	      coverage stays wired rather than re-deriving it)
//	AC2 "a negative verdict with green artifacts but no explanation is still
//	     detected as verdict-incoherence, except the fully-verified late-write
//	     case reconciles to PASS"
//	    → C1587_002 (pre-existing GREEN, same file — the forged-vs-reconciled
//	      branch, TestDetectVerdictIncoherence_ForgedVerdict_Halts +
//	      TestDetectVerdictIncoherence_ReconcileUsesFullVerify)
//	AC3 "malformed or missing artifacts cannot be laundered into reconciliation"
//	    → C1587_003 (pre-existing GREEN, same file —
//	      TestDetectVerdictIncoherence_ReconcileUsesFullVerify's OK=false arm)
//
// The concrete pipeline-blocker shape this cycle repairs — a DEFERRED
// quota-exhaustion abort minting its OWN false-negative "verdict" via
// failure-learning bookkeeping before the coherence floor ever runs — is
// pinned by C1587_004..007, driving the real dispatch()/RunCycle chain:
//
//	C1587_004 no FailedRecord is appended on the all-85 (DEFERRED) arm
//	C1587_005 no P0 carryover todo is queued on the all-85 (DEFERRED) arm
//	C1587_006 the retro runner is never force-run as a learning side effect
//	C1587_007 (NEGATIVE, anti-no-op) a single exit=85 attempt with a
//	          differently-shaped (non-85) sibling — NOT the all-families
//	          signature — still learns normally: FailedRecord appended AND
//	          the carryover todo queued. A guard that swallows every failure
//	          passes C1587_004..006 and dies here.
//	C1587_008 the eval file for this task passes the SSOT quality checker
//	          over a non-empty score_cap set (a vacuous eval PASSes for free)
//
// Adversarial axes: negative (C1587_007 — the fix must not become a blanket
// learning suppression), edge (the multiply-%w-wrapped ErrAllFamiliesExhausted
// sentinel matched via errors.Is, exercised inside C1587_004..006's underlying
// core test), semantic (verdict-coherence-at-finalization vs
// bookkeeping-before-classification are two distinct seams, not one behavior
// restated).
//
// No source-grep predicates (cycle-85 rule): every predicate here runs the
// real production dispatch/finalization chain as a subprocess `go test` and
// asserts on the named PASS marker (a bare exit 0 would hide a renamed or
// skipped test) or the eval-quality checker's own verdict. Every invocation
// names ONE package and is narrowed with -run (flaky-predicate-shape rule: no
// `/...` sweep, no unnarrowed ./internal/core).
package cycle1587

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/evalqualitycheck"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	evalSlug = "pipeline-defect-pipeline-blocker-cycle1582"
)

// runCoreTest runs exactly one named test (or "Test/subtest") in internal/core
// and returns its verbose output. One named package, always -run-narrowed —
// never a /... sweep.
func runCoreTest(t *testing.T, name string) string {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", "^"+topLevel(name)+"$", corePkg)
	if code != 0 || err != nil {
		t.Errorf("go test -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			topLevel(name), corePkg, code, err, stdout, stderr)
	}
	marker := "--- PASS: " + name
	if !strings.Contains(stdout, marker) {
		t.Errorf("%s did not report PASS (renamed, skipped, or not run)\nstdout:\n%s", name, stdout)
	}
	return stdout
}

// topLevel strips a "/subtest" suffix so `-run` anchors the top-level test
// name (Go's -run matches "TestFoo/sub" against each path segment, but the
// anchor must name the top-level test to let subtests run at all).
func topLevel(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i]
	}
	return name
}

// C1587_001 (AC1, pre-existing GREEN regression pin): a negative verdict
// EXPLAINED by a runtime-minted SubstantiveError (audit-phase diagnosed
// gate-downgrade, cycles-930/931/932; ship-phase rejection, cycle-1329) must
// never trigger verdict-incoherence. Binds the coherence floor's own
// regression suite so a future edit cannot silently regress it without also
// failing this cycle's audit.
func TestC1587_001_explained_negative_verdict_stays_coherent(t *testing.T) {
	runCoreTest(t, "TestDetectVerdictIncoherence_DiagnosedGateFail_NoHalt")
	runCoreTest(t, "TestDetectVerdictIncoherence_ShipPhaseExplainedFail_NoHalt")
}

// C1587_002 (AC2): an UNEXPLAINED negative verdict with green artifacts is
// still detected as verdict-incoherence and halts — except the fully-verified
// clean-exit-late-write race, which self-heals to a reconcile (nil signal).
func TestC1587_002_unexplained_negative_detected_except_verified_latewrite(t *testing.T) {
	runCoreTest(t, "TestDetectVerdictIncoherence_ForgedVerdict_Halts")
	runCoreTest(t, "TestDetectVerdictIncoherence_ReconcileUsesFullVerify")
}

// C1587_003 (AC3, anti-laundering): a PASS-sentinel-tagged report that does
// NOT fully verify (malformed deliverable) must never reconcile — proven by
// the same ReconcileUsesFullVerify test's OK=false arm, and reinforced by the
// workspace-file trust-boundary case (an agent-writable fail-reason file alone
// cannot rescue a genuine forgery).
func TestC1587_003_malformed_artifact_never_laundered_into_reconciliation(t *testing.T) {
	runCoreTest(t, "TestDetectVerdictIncoherence_ReconcileUsesFullVerify")
	runCoreTest(t, "TestDetectVerdictIncoherence_WorkspaceReasonFileAlone_StillHalts")
}

// C1587_004: on the all-CLI-families-quota-exhausted (DEFERRED) dispatch arm,
// no FailedRecord is appended to state.FailedAt — a DEFERRED resume checkpoint
// is not a diagnosed failure.
func TestC1587_004_deferred_arm_appends_no_failed_record(t *testing.T) {
	runCoreTest(t, "TestDispatch_AllFamiliesExhausted_NoFailureLearning/no_FailedRecord_appended")
}

// C1587_005: no P0 "cycle-<N>-failed-<phase>" carryover todo is queued on the
// DEFERRED arm — the exact cycle-1582 dossier root cause (a spurious todo
// competing with real work on every quota wall).
func TestC1587_005_deferred_arm_queues_no_carryover_todo(t *testing.T) {
	runCoreTest(t, "TestDispatch_AllFamiliesExhausted_NoFailureLearning/no_P0_carryover_todo_queued")
}

// C1587_006 (the strongest direct proof): the retro runner is never
// force-invoked as a failure-learning side effect on the DEFERRED arm, driven
// both through the counting-fake unit path and the full RunCycle chain.
func TestC1587_006_deferred_arm_never_force_runs_retro(t *testing.T) {
	runCoreTest(t, "TestDispatch_AllFamiliesExhausted_NoFailureLearning/retro_runner_never_invoked_for_learning")
	runCoreTest(t, "TestRunCycle_AllFamiliesExhausted_DoesNotDispatchRetro")
	runCoreTest(t, "TestRecordFailureLearning_MultiplyWrappedExhausted_SkipsRetro")
}

// C1587_007 (NEGATIVE / anti-no-op — the sharpest signal in this cycle): a
// single exit=85 attempt followed by a differently-shaped (non-85) sibling
// failure is NOT the all-families-exhausted signature. Normal failure-learning
// — a FailedRecord AND the P0 carryover todo AND exactly one retro dispatch —
// must still fire exactly as before. A fix that short-circuits on "any exit=85
// seen" instead of the typed ErrAllFamiliesExhausted sentinel passes
// C1587_004..006 and fails here.
func TestC1587_007_non_exhaustion_failure_still_learns_normally(t *testing.T) {
	runCoreTest(t, "TestDispatch_SingleFamily85WithSibling_FailureLearningUnchanged")
}

// C1587_008 (AC coverage rigor): the task's eval file
// (.evolve/evals/pipeline-defect-pipeline-blocker-cycle1582.md) passes the SSOT
// quality checker over a non-empty score_cap set — a vacuous eval with no
// graded criteria PASSes for free, which the contract forbids.
func TestC1587_008_eval_file_passes_quality_check(t *testing.T) {
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
}
