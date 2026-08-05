//go:build acs

// Package cycle1331 materialises the cycle-1331 acceptance criteria for this
// fleet lane's two assigned todos (fleet_scope: audit-warn-prescription-gate,
// percycle-audit-apicover-newexport-parity).
//
// Investigation (tdd phase, this cycle) found BOTH todos already have their
// production fix committed at HEAD (git log: "052ee69f salvage snapshot
// (ADR-0076 continuation-on-fail)") — a prior lane's salvage landed
// phasecontract.Failure.Prescription + emitDefectLedger's prescription sourcing
// + a full white-box test suite (defect_ledger_prescription_test.go) BEFORE
// this lane started. Task 1's ACs are therefore pre-existing GREEN, not RED —
// see test-report.md for the full "cannot manufacture RED for already-shipped
// work" reasoning. Task 2's untested edge (scout Finding 4: a new exported
// symbol landing in an EXISTING enforced package via a brand-new file) had NO
// regression test anywhere in the tree; this cycle adds one
// (ciparity_newexport_test.go) that also passes immediately, confirming the
// scout's Hypothesis 2 ("the code path is correct in the common case; the risk
// is an untested edge") rather than surfacing a defect.
//
// Predicate strategy — each predicate shells `go test -run <pattern> -count=1
// -v <pkg>` over the DEFAULT (non-acs) suite and requires a `--- PASS: <name>`
// line per named test (the cycle-997/cycle-1329 SubprocessOutput precedent).
// Asserting on the PASS line, not merely exit 0, is essential: a pattern
// matching zero tests exits 0 with "no tests to run", so a renamed/deleted
// test would otherwise false-GREEN.
package cycle1331

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -count=1 -v
// pkg` in the DEFAULT build suite (no -tags) and requires EVERY name to
// print a `--- PASS: <name>` line. -count=1 defeats the test cache so a stale
// prior result can never masquerade as evidence this cycle's diff makes them
// pass.
func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-count=1", "-v", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("default-suite test %s did NOT pass in %s "+
				"(missing, failing, or a build-compile error). exit=%d\ncombined go-test output:\n%s",
				name, pkg, code, out)
		}
	}
}

// assertVetClean shells `go vet pkg` for ONE named package (never a
// whole-repo `/...` sweep — the flaky-predicate-shape house rule) and
// requires exit 0.
func assertVetClean(t *testing.T, pkg string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", pkg)
	if code == -1 {
		t.Fatalf("go vet failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	if code != 0 {
		t.Errorf("go vet %s exited %d, want 0 clean\nstdout:\n%s\nstderr:\n%s", pkg, code, stdout, stderr)
	}
}

// -- Task 1: audit-warn-prescription-gate (pre-existing GREEN) -------------

// TestC1331_001_WarnPrescriptionMintsAddressableLedgerEntry — AC: a ciparity
// gate's own WARN sentinel carrying a structured `prescription` and zero
// `defects` must still mint an OPEN, id-bearing defect-ledger.json entry
// (positive case), and an empty prescription array must mint nothing
// (negative anti-vacuous-widen case) — both halves of
// TestDefectLedger_WarnPrescription.
func TestC1331_001_WarnPrescriptionMintsAddressableLedgerEntry(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestDefectLedger_WarnPrescription",
	)
}

// TestC1331_002_InheritedPrescriptionConsumedByNextCycle — AC: an inherited
// OPEN prescription-sourced ledger entry blocks a continuation's PASS via the
// SAME reconcile/evidence path FAIL-sourced defects already use (unaccounted
// blocks; FIXED-with-resolving-evidence unblocks; unverifiable evidence still
// blocks) — proving the record is genuinely "readable by a subsequent cycle"
// per Task 1's verifiableBy, not merely persisted.
func TestC1331_002_InheritedPrescriptionConsumedByNextCycle(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestReconcile_WarnPrescriptionBlocks",
	)
}

// TestC1331_003_PrescriptionlessWarnBehaviorUnchanged — AC: an ordinary WARN
// with no `prescription` field (the pre-fix, still-majority shape) must mint
// no ledger entry and leave the WARN verdict unchanged — the explicit
// regression guard against widening emitDefectLedger's trigger into every
// narrative WARN (scout AC: "existing WARN/diagnostic behavior unchanged").
func TestC1331_003_PrescriptionlessWarnBehaviorUnchanged(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestAudit_WarnWithoutPrescription_NoRegression",
	)
}

// -- Task 2: percycle-audit-apicover-newexport-parity -----------------------

// TestC1331_004_NewExportViaNewFileInExistingPackageCaught — AC: a new
// exported symbol landing in an EXISTING enforced package via a brand-new
// file (handoff files_new, not files_modified) must be caught by
// apicoverEnforceChangedDefault exactly as CI's whole-repo `apicover
// -enforce` would — the untested edge scout Finding 4 named. New this cycle
// (ciparity_newexport_test.go); passes immediately, confirming parity rather
// than surfacing a defect (scout Hypothesis 2).
func TestC1331_004_NewExportViaNewFileInExistingPackageCaught(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestApicoverEnforceChangedDefault_NewExportViaNewFileInExistingPackage_CaughtByGate",
	)
}

// TestC1331_005_NewExportViaNewFileNotMisroutedToGraduationGate — negative
// boundary half of AC above: the identical fixture must be a no-op for
// apicoverNewPackageGraduationDefault, because ./internal/p is already
// enforced — this shape belongs to the touched∩enforced gate, not the
// new-package graduation gate. Without this split a silent mis-route between
// the two gates would go completely unflagged by either.
func TestC1331_005_NewExportViaNewFileNotMisroutedToGraduationGate(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestApicoverEnforceChangedDefault_NewExportViaNewFile_NotGraduationGate",
	)
}

// TestC1331_006_GoVetCleanOnTouchedPackages — house rule: go vet must stay
// clean on every package this cycle's new test file touches. Scoped
// per-package (never a whole-repo `/...` sweep) per the flaky-predicate-shape
// rule.
func TestC1331_006_GoVetCleanOnTouchedPackages(t *testing.T) {
	assertVetClean(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/...")
}
