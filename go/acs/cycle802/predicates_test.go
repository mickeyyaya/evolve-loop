//go:build acs

// Package cycle802 materializes the cycle-802 acceptance criteria for this
// fleet lane's sole committed inbox item, retro-bridge-timeout-width10
// (scout-report.md Selected Tasks 1-4; per R9.3 no predicates bind to any
// other lane's items or to the Deferred TestConcurrentNonFloorLaunchesBounded
// criterion).
//
// AC map (1:1, from scout-report.md Selected Tasks verifiableBy + Acceptance
// Criteria Summary):
//
//	AC1 non-floor phase FAIL/WARN never overwrites an already-recorded
//	    floor-derived FinalVerdict (Task 1: floor-gated-final-verdict)
//	    → C802_001 runs TestNonFloorPhaseFailure_DoesNotOverrideFloorVerdict.
//	AC2 non-floor phase failure after audit already FAILed leaves
//	    FinalVerdict FAIL (no accidental "recovery") (Task 1)
//	    → C802_002 runs TestNonFloorPhaseFailure_FailAudit_StaysFail.
//	AC3 a FLOOR phase's own failure remains cycle-fatal — the guard must
//	    not accidentally shield floor phases too (Task 1)
//	    → C802_003 runs TestFloorPhaseFailure_RemainsCycleFatal.
//	AC4 the resume dispatch loop (resume.go:324) carries the identical
//	    guard, closing the --resume storm-recurrence gap (Task 2:
//	    floor-gated-final-verdict-resume-parity)
//	    → C802_004 runs TestResumeNonFloorPhaseFailure_DoesNotOverrideFloorVerdict.
//	AC5 contract exhaustion (unparseable verdict after retries) on a
//	    non-floor phase degrades to SKIPPED+WARN via the same floor gate,
//	    subsuming advisory-phase-contract-degrade (Task 3:
//	    contract-exhaustion-degrades-non-floor)
//	    → C802_005 runs TestContractExhaustion_NonFloorPhase_DegradesToSkippedWarn.
//	AC6 skipped/degraded non-floor phases are surfaced in the dossier via
//	    a durable CycleResult.SkippedPhases[] field, never silently dropped
//	    (Task 1 dossier surfacing)
//	→ C802_006 runs TestDossier_RecordsSkippedPhases.
//	AC7 retrospective.json and memo.json declare sandbox.allow_network
//	    true so the runtime stops silently forcing it and emitting the
//	    noisy WARN (Task 4: retro-memo-allow-network-honest)
//	    → C802_007 is a config-check waiver (declarative JSON field, not
//	      logic) reading both profiles directly.
//
// Adversarial axes: negative (AC2/AC3 pin the cases the guard must NOT
// change — a naive "always keep first PASS" fix would break AC3), edge
// (AC5 exhaustion after retries, not first failure), semantic (AC1 floor
// non-overwrite, AC3 floor-fatal, AC6 dossier surfacing, and AC7 config
// honesty are four distinct behaviors, not one behavior restated). No
// source-grep predicates over logic files (cycle-85 rule): AC1-AC6 each
// execute the system under test as a subprocess; AC7 is the declared
// config-check exception (see go/acs/README.md predicate-quality table).
package cycle802

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

// runCoreTest runs one named Test in internal/core under -race and requires
// an explicit verbose PASS marker for it — exit 0 alone would also cover the
// "0 tests matched" case (a renamed/removed test), which must fail the
// predicate, not pass it.
func runCoreTest(t *testing.T, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-race", "-count=1", "-v", "-run", "^"+name+"$", corePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -race -run %s %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			name, corePkg, code, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Errorf("core test %s did not report PASS (missing, renamed, or not matched)", name)
	}
}

// AC1: non-floor phase failure must not clobber a floor-derived FinalVerdict.
func TestC802_001_non_floor_failure_never_overrides_floor_verdict(t *testing.T) {
	runCoreTest(t, "TestNonFloorPhaseFailure_DoesNotOverrideFloorVerdict")
}

// AC2: non-floor phase failure after audit already FAILed must not flip
// FinalVerdict back to anything other than FAIL — pins the "stays FAIL"
// direction the guard must also preserve (not just "stays PASS").
func TestC802_002_non_floor_failure_after_fail_audit_stays_fail(t *testing.T) {
	runCoreTest(t, "TestNonFloorPhaseFailure_FailAudit_StaysFail")
}

// AC3: a FLOOR phase's own failure (e.g. audit FAIL) must remain
// cycle-fatal — the negative control proving the fix scopes to non-floor
// phases only, not "never let anything change FinalVerdict."
func TestC802_003_floor_phase_failure_remains_cycle_fatal(t *testing.T) {
	runCoreTest(t, "TestFloorPhaseFailure_RemainsCycleFatal")
}

// AC4: resume-path parity (resume.go:324) — the identical guard, exercised
// through the resume dispatcher, so `evolve loop --resume` cannot reintroduce
// the storm on recovery batches.
func TestC802_004_resume_path_non_floor_failure_never_overrides_floor_verdict(t *testing.T) {
	runCoreTest(t, "TestResumeNonFloorPhaseFailure_DoesNotOverrideFloorVerdict")
}

// AC5: contract exhaustion (non-canonical verdict after retries exhausted)
// on a non-floor phase degrades to SKIPPED+WARN via the same floor gate,
// rather than aborting the cycle or clobbering FinalVerdict.
func TestC802_005_contract_exhaustion_non_floor_degrades_to_skipped_warn(t *testing.T) {
	runCoreTest(t, "TestContractExhaustion_NonFloorPhase_DegradesToSkippedWarn")
}

// AC6: skipped/degraded non-floor phases must be surfaced in the dossier
// (CycleResult.SkippedPhases[]), not silently dropped once FinalVerdict stops
// reflecting them.
func TestC802_006_dossier_records_skipped_phases(t *testing.T) {
	runCoreTest(t, "TestDossier_RecordsSkippedPhases")
}

// AC7 (config-check waiver — declarative JSON field, not logic under test):
// retrospective.json and memo.json must declare sandbox.allow_network true
// so the runtime stops silently forcing it and emitting the noisy WARN.
//
// acs-predicate: config-check
func TestC802_007_retro_memo_allow_network_honest(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, profile := range []string{"retrospective.json", "memo.json"} {
		path := filepath.Join(root, ".evolve", "profiles", profile)
		if !acsassert.JSONFieldEquals(t, path, "sandbox.allow_network", true) {
			t.Errorf("%s: sandbox.allow_network must be declared true", profile)
		}
	}
}
