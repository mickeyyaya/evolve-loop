//go:build acs

// Package cycle1440 materialises the cycle-1440 acceptance criteria for the
// three tasks triage committed to this lane:
//
//	carryover-pass-retirement            — PASS-closeout deletion path so a
//	                                       committed/fingerprint-matched carryover
//	                                       id retires instead of persisting forever.
//	deterministic-stage-refusal-router   — two-strikes-same-pathspec rule so a
//	                                       consecutive identical GIT_STAGE_FAILED
//	                                       refusal classifies deterministic instead
//	                                       of burning the whole retry budget
//	                                       (cycle-1365).
//	fingerprint-normalizer-path-variance — path/attempt-denominator normalization
//	                                       so one recurring defect fingerprints as
//	                                       ONE for the 3-strike breaker.
//
// Predicate strategy. Every predicate EXERCISES the production seam by running
// the named RED tests — never a source-grep of production text (the cycle-85
// degenerate-predicate ban): a predicate asserting "failure_digest.go now
// contains a path regex" would pass on a cosmetic edit that normalizes nothing.
// Each run is narrowed with `-run` (never a whole ./internal/core sweep — the
// flaky-predicate rule, cycles 1173/1175/1178) and demands an explicit
// `--- PASS: <name>` line per test, so a rename or a skip can never satisfy a
// predicate on exit code alone.
//
// Both NEGATIVE halves are load-bearing and are named here explicitly:
// AC2's `DifferentPathspecStaysTransient` refutes a rule that merely counts
// refusals, and AC3's `DistinctDefectsStayDistinct` refutes an over-broad
// normalizer that collapses two different defects into one fingerprint.
package cycle1440

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// Packages under test, addressed by full import path so the predicate resolves
// regardless of the cwd `go test` hands this package.
const (
	corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	shipPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

// runNamedTests runs the named tests (fresh, verbose, `-run`-narrowed) and
// requires an explicit "--- PASS: <name>" for every wantPass. Exit 0 alone never
// satisfies a predicate: a renamed, skipped, or never-authored test emits no
// PASS line.
func runNamedTests(t *testing.T, pkg, runExpr string, wantPass []string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-run", runExpr, "-v", pkg)
	if code != 0 || err != nil {
		t.Fatalf("go test -run %q %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			runExpr, pkg, code, err, stdout, stderr)
	}
	for _, name := range wantPass {
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("test %s did not report PASS (renamed, skipped, or not authored)\nstdout:\n%s", name, stdout)
		}
	}
}

// -----------------------------------------------------------------------------
// AC1 — carryover-pass-retirement.
//
// AC1a is the pure seam (retire by committed id, retire the cross-cycle
// fingerprint variant with it, leave everything else in order, never mutate the
// caller's slice). AC1b is the WIRING PROOF: the production PASS closeout
// (promoteInbox) must reach that seam and remove the entry from state.json, and
// must obey the same landing gate promotion already rides. A seam whose only
// caller is a test is dead code, so AC1b is the one that actually gates.
// -----------------------------------------------------------------------------

func TestC1440_001_CarryoverRetiresOnCommittedID(t *testing.T) {
	runNamedTests(t, corePkg, "^TestRetireCarryoverTodos_", []string{
		"TestRetireCarryoverTodos_CommittedIDRetires",
		"TestRetireCarryoverTodos_FingerprintVariantRetires",
		"TestRetireCarryoverTodos_UnmatchedSurvivesInOrder",
		"TestRetireCarryoverTodos_EdgeInputs",
		"TestRetireCarryoverTodos_DoesNotMutateInput",
	})
}

func TestC1440_002_CarryoverRetirementWiredIntoPassCloseout(t *testing.T) {
	runNamedTests(t, shipPkg, "^TestPromoteInbox_(LandedPassRetiresCommittedCarryover|UnlandedPassKeepsCarryover|NoStateFileIsNoOp)$", []string{
		"TestPromoteInbox_LandedPassRetiresCommittedCarryover",
		"TestPromoteInbox_UnlandedPassKeepsCarryover",
		"TestPromoteInbox_NoStateFileIsNoOp",
	})
}

// -----------------------------------------------------------------------------
// AC2 — deterministic-stage-refusal-router. Driven through the real staging
// seam (stageExplicitPaths) with a scripted refusing `git add`, so the predicate
// reads the CLASS the production code stamps on the ShipError.
// -----------------------------------------------------------------------------

func TestC1440_003_SecondIdenticalStageRefusalIsDeterministic(t *testing.T) {
	runNamedTests(t, shipPkg, "^TestStageRefusal_", []string{
		"TestStageRefusal_FirstStrikeStaysTransient",
		"TestStageRefusal_SecondSamePathspecIsDeterministic",
		"TestStageRefusal_DifferentPathspecStaysTransient",
		"TestStageRefusal_SeparateWorkspacesDoNotShareStrikes",
		"TestStageRefusal_NoWorkspaceStaysTransient",
	})
}

// -----------------------------------------------------------------------------
// AC3 — fingerprint-normalizer-path-variance. The two folding rules PLUS the
// two guards (distinct defects stay distinct; the existing narrative/duration
// pins stay green) run as one predicate: the folding half alone is satisfiable
// by a normalizer that returns a constant.
// -----------------------------------------------------------------------------

func TestC1440_004_FingerprintFoldsPathAndAttemptVariance(t *testing.T) {
	runNamedTests(t, corePkg, "^TestNormalizeReasonForFingerprint_", []string{
		"TestNormalizeReasonForFingerprint_CycleNumberedPathsFold",
		"TestNormalizeReasonForFingerprint_AttemptDenominatorFolds",
		"TestNormalizeReasonForFingerprint_DistinctDefectsStayDistinct",
		"TestNormalizeReasonForFingerprint_ExistingPinsStayGreen",
		"TestNormalizeReasonForFingerprint_TouchesOnlyTheNarrativeToken",
	})
}
