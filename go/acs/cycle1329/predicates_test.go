//go:build acs

// Package cycle1329 materialises the cycle-1329 acceptance criteria for the
// fleet-scoped todo `audit-warn-prescription-gate`: give the audit-side
// new-package graduation gate (`apicoverNewPackageGraduationDefault`,
// go/internal/phases/audit/ciparity.go:647) the same copy-pasteable
// prescription the build-entry seam already emits
// (`graduationPrescription`, go/internal/core/phase_bindings_graduation.go:81)
// instead of the terse "add it + an apicover_named_test.go" sentence — by
// relocating that helper into the shared `ciparity` package (exported as
// `GraduationPrescription`) both seams already import.
//
// Predicate strategy — each predicate shells `go test -run <pattern> -count=1
// -v <pkg>` over the DEFAULT (non-acs) suite and requires a `--- PASS: <name>`
// line per named test (the cycle-997 SubprocessOutput precedent). Asserting on
// the PASS line, not merely exit 0, is essential: a pattern matching zero
// tests exits 0 with "no tests to run", so a still-missing/still-unwired test
// would otherwise false-GREEN. This mirrors the reachability-probe /
// caller-proof house rule: these predicates require the RELOCATED seams to be
// actually reached, not merely present as dead code.
//
//   - 001 binds go/internal/ciparity/graduation_prescription_test.go (this
//     TDD phase's own new white-box tests) — the relocated, exported
//     GraduationPrescription helper, covering AC3 (the "..." pattern branch
//     preserved) plus positive/negative/semantic diversity.
//   - 002 binds go/internal/phases/audit/ciparity_newpkg_test.go's new
//     TestApicoverNewPkgGraduationDefault_OffenderIncludesPrescriptiveFix —
//     AC1: the audit offender line must carry the literal .apicover-enforce
//     append line and apicover_named_test.go path.
//   - 003 binds the PRE-EXISTING build-entry regression suite
//     (go/internal/core/phase_bindings_graduation_test.go) — AC2: the
//     relocation must not require editing that file, and its abort_reason
//     content must stay byte-identical (already green today; this predicate
//     keeps it green through the refactor without letting it silently break).
//   - 004 is AC4: `go vet` must stay clean on every package this task
//     touches. Scoped to the three touched packages individually (not a
//     whole-repo `/...` sweep) per the flaky-predicate-shape house rule —
//     `go vet` is a static check (no test execution, no timing/PID/process
//     hazard), so per-package invocation is both correct-scoped and cheap.
//     Currently RED: `go vet ./internal/ciparity/...` fails to compile
//     graduation_prescription_test.go (undefined: GraduationPrescription).
package cycle1329

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// assertDefaultSuiteTestsPass shells `go test -run '^(names)$' -count=1 -v
// pkg` in the DEFAULT build suite (no -tags) and requires EVERY name to
// print a `--- PASS: <name>` line. -count=1 defeats the test cache so a
// stale prior result can never masquerade as evidence this cycle's diff
// makes them pass.
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

// TestC1329_001_CiparityGraduationPrescriptionExportedAndCorrect — AC3: the
// relocated, exported ciparity.GraduationPrescription must exist and
// preserve every behavior the build-seam original had, including the
// recursive "..."-pattern branch. Currently RED: GraduationPrescription is
// undefined in package ciparity (a compile failure), because the helper
// still lives unexported in package core.
func TestC1329_001_CiparityGraduationPrescriptionExportedAndCorrect(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/ciparity",
		"TestGraduationPrescription_EmitsAppendLineAndTestPath",
		"TestGraduationPrescription_EmptyInputReturnsEmptyString",
		"TestGraduationPrescription_PatternSuffixSkipsBogusPath",
		"TestGraduationPrescription_MultiplePackagesEachGetOwnBlock",
	)
}

// TestC1329_002_AuditOffenderCarriesPrescriptiveFix — AC1: the audit
// graduation gate's offender line must include the literal
// .apicover-enforce append line and apicover_named_test.go path, not the
// generic "add it" sentence. Currently RED: the offender string does not
// contain either literal.
func TestC1329_002_AuditOffenderCarriesPrescriptiveFix(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit",
		"TestApicoverNewPkgGraduationDefault_OffenderIncludesPrescriptiveFix",
	)
}

// TestC1329_003_BuildSeamRegressionStaysGreenThroughRelocation — AC2: the
// PRE-EXISTING build-entry graduation test suite must keep passing, byte
// for byte, without any edit to phase_bindings_graduation_test.go, once
// buildGraduationCheck calls the relocated ciparity.GraduationPrescription
// instead of a local unexported copy. Already green today (nothing moved
// yet); this predicate is the regression guard that keeps it green once the
// relocation lands, per the "doNotModifyTests" handoff contract.
func TestC1329_003_BuildSeamRegressionStaysGreenThroughRelocation(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/core",
		"TestBuildGraduationCheck",
		"TestRecordAndBranch_BuildGraduationGuardAborts",
		"TestRecordAndBranch_BuildGraduationGuardEnrolledProceeds",
	)
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

// TestC1329_004_GoVetCleanOnTouchedPackages — AC4: go vet must stay clean
// on every package this task touches after the relocation. Currently RED:
// go vet ./internal/ciparity/... fails to compile
// graduation_prescription_test.go (undefined: GraduationPrescription).
func TestC1329_004_GoVetCleanOnTouchedPackages(t *testing.T) {
	assertVetClean(t, "github.com/mickeyyaya/evolve-loop/go/internal/ciparity/...")
	assertVetClean(t, "github.com/mickeyyaya/evolve-loop/go/internal/core/...")
	assertVetClean(t, "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit/...")
}
