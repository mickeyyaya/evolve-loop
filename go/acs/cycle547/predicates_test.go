//go:build acs

// Package cycle547 materialises the cycle-547 acceptance criteria for the
// three triage-committed (`## top_n`) tasks (scout-report.md "Selected
// Tasks"; no separate triage-report.md this cycle — scout committed all
// three, see scout-report.md's Decision Trace task_proposals):
//
//  1. fleet-min-width-lane-fallback     — cmd/evolve wave dispatch: a
//     quota/budget-shrunk-to-<=1-lane wave with >=1 disjoint candidate must
//     dispatch isolated (1 lane), never fall to the leak-prone sequential
//     path.
//  2. memo-phase-routing-repair          — internal/phasespec: the built-in
//     optional `memo` phase's activation overlay must route without
//     warnings; the two-tier naming floor's new built-in-name exemption must
//     stay scoped to OPTIONAL built-ins only.
//  3. apicover-new-package-graduation-gate — internal/ciparity +
//     internal/phases/audit: a changed go/internal/<pkg> absent from
//     .apicover-enforce must FAIL the audit gate; go/cmd/... changes and
//     already-graduated packages must not be flagged.
//
// Predicate strategy (mirrors cycle499/503/504/507): BEHAVIORAL predicates
// drive the system under test through its in-package RED tests via
// subprocess `go test`, asserting a non-degenerate pass (requireTestsRan
// closes the cycle-85 "no tests to run" trap) — never a source grep. The
// in-package tests were authored by the TDD engineer this cycle:
//
//	cmd/evolve/cmd_loop_wave_minwidth_test.go        (Task 1)
//	internal/phasespec/validate_builtin_exempt_test.go (Task 2)
//	internal/ciparity/newpkg_test.go                 (Task 3, pure fn)
//	internal/phases/audit/ciparity_newpkg_test.go    (Task 3, gate wiring)
//
// The Builder implements production code ONLY (the seams named in those
// files); it must not modify the tests.
package cycle547

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	phasespecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	ciparityPkg  = "github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	auditPkg     = "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
)

// runGoTest runs `go test` on pkg filtered by runFilter, returning combined
// output + exit code. Behavioral predicates invoke the system under test
// through its own in-package tests — no source-grep gaming.
func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

// requireTestsRan closes the degenerate-predicate trap: `go test -run X` with
// no matching test exits 0 with "no tests to run", which would green a
// predicate on unwritten (or renamed) work.
func requireTestsRan(t *testing.T, out string, min int) {
	t.Helper()
	if strings.Contains(out, "no tests to run") {
		t.Errorf("no tests matched the -run filter (\"no tests to run\") — required tests are unwritten or renamed")
		return
	}
	if got := strings.Count(out, "=== RUN"); got < min {
		t.Errorf("only %d test(s) ran, need >= %d", got, min)
	}
}

// TestC547_001_ForceOneLaneDispatch_IsolatedNotSequential (Task 1, AC1/AC3):
// forceOneLaneDispatch dispatches a >=1-candidate backlog through the SAME
// isolated launcher path (not sequential), stays false+no-launch on a
// genuinely empty backlog, and never bypasses the S3 preflight guard.
// Drives cmd_loop_wave_minwidth_test.go. RED today: forceOneLaneDispatch
// undefined (package main test build fails).
func TestC547_001_ForceOneLaneDispatch_IsolatedNotSequential(t *testing.T) {
	out, code := runGoTest(t,
		"TestForceOneLaneDispatch_DispatchesIsolatedWaveWhenCandidateExists|TestForceOneLaneDispatch_EmptyBacklogStaysFalseNoLauncherInvoked|TestForceOneLaneDispatch_PreflightRefusalNeverPlansNorLaunches",
		cmdEvolvePkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("fleet min-width lane fallback is red (exit=%d) — forceOneLaneDispatch missing or wrong\n%s", code, out)
	}
}

// TestC547_002_ShouldRunWave_CountGateUnloosened (Task 1, AC2: "fleet.count=1
// legacy path untouched") — guards against the wrong fix: shouldRunWave must
// stay Count>1, not be loosened to Count>=1. Drives
// cmd_loop_wave_minwidth_test.go.
func TestC547_002_ShouldRunWave_CountGateUnloosened(t *testing.T) {
	out, code := runGoTest(t, "TestShouldRunWave_CountOneOrZeroStillFalse", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("shouldRunWave's Count>1 gate regressed (exit=%d) — a fleet.count=1 static config must NOT be routed through the wave path\n%s", code, out)
	}
}

// TestC547_003_MemoOverlayRoutesWithoutWarning (Task 2, all 4 AC clauses
// except ADR-0058 stripping, which is existing regression coverage per
// validate_builtin_exempt_test.go's header comment): the built-in-name
// exemption is scoped to optional built-ins, rejects a genuinely new
// single-word name, rejects a non-optional built-in-name hijack attempt
// (audit), and the real memo overlay routes end to end with zero warnings.
// Drives internal/phasespec/validate_builtin_exempt_test.go. RED today:
// ValidateUserSpecWithCatalog undefined and ApplyUserRouting has the wrong
// arity (package phasespec test build fails).
func TestC547_003_MemoOverlayRoutesWithoutWarning(t *testing.T) {
	out, code := runGoTest(t,
		"TestValidateUserSpecWithCatalog_ExemptsOptionalBuiltinName|TestValidateUserSpecWithCatalog_RejectsGenuineNewSingleWordName|TestValidateUserSpecWithCatalog_RejectsNonOptionalBuiltinNameOverlay|TestApplyUserRouting_RoutesBuiltinNameOverlayWithoutWarning",
		phasespecPkg)
	requireTestsRan(t, out, 4)
	if code != 0 {
		t.Errorf("memo-phase-routing-repair is red (exit=%d) — ValidateUserSpecWithCatalog/ApplyUserRouting missing or wrong\n%s", code, out)
	}
}

// TestC547_004_NewUngraduatedPackages_PureFn (Task 3, pure-function contract):
// NewUngraduatedPackages flags a new changed internal package absent from
// .apicover-enforce, leaves already-graduated packages unflagged, never
// flags go/cmd/..., and dedupes+sorts. Drives internal/ciparity/newpkg_test.go.
// RED today: NewUngraduatedPackages undefined (package ciparity test build
// fails).
func TestC547_004_NewUngraduatedPackages_PureFn(t *testing.T) {
	out, code := runGoTest(t,
		"TestNewUngraduatedPackages_FlagsChangedInternalPkgAbsentFromEnforceList|TestNewUngraduatedPackages_AlreadyGraduatedPackagesNotFlagged|TestNewUngraduatedPackages_CmdPackagesNeverFlagged|TestNewUngraduatedPackages_DedupesAndSorts",
		ciparityPkg)
	requireTestsRan(t, out, 4)
	if code != 0 {
		t.Errorf("apicover new-package graduation pure function is red (exit=%d) — NewUngraduatedPackages missing or wrong\n%s", code, out)
	}
}

// TestC547_005_ApicoverNewPkgGraduationGate_WiredIntoAudit (Task 3, gate
// wiring — the anti-dead-code predicate: a pure function nothing calls is
// worthless, exactly the warnship_apicover_ci_gap trap this task exists to
// close for the 3rd time). Drives internal/phases/audit/ciparity_newpkg_test.go.
// RED today: Config.CheckApicoverNewPkgGraduation /
// apicoverNewPackageGraduationDefault undefined (package audit test build
// fails).
func TestC547_005_ApicoverNewPkgGraduationGate_WiredIntoAudit(t *testing.T) {
	out, code := runGoTest(t,
		"TestApicoverNewPkgGraduation_OffendersFailAudit|TestApicoverNewPkgGraduationDefault_NoUngraduatedPackages_NoOp|TestApicoverNewPkgGraduationDefault_UngraduatedPackageFlagged|TestApicoverNewPkgGraduationDefault_CmdChangeNotFlagged",
		auditPkg)
	requireTestsRan(t, out, 4)
	if code != 0 {
		t.Errorf("apicover new-package graduation gate is red or not wired into audit (exit=%d)\n%s", code, out)
	}
}
