//go:build acs

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

func runGoTest(t *testing.T, runFilter, pkg string) (out string, code int) {
	t.Helper()
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test", "-count=1", "-v", "-run", runFilter, pkg)
	return stdout + "\n" + stderr, code
}

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

func TestC547_001_ForceOneLaneDispatch_IsolatedNotSequential(t *testing.T) {
	out, code := runGoTest(t,
		"TestForceOneLaneDispatch_DispatchesIsolatedWaveWhenCandidateExists|TestForceOneLaneDispatch_EmptyBacklogStaysFalseNoLauncherInvoked|TestForceOneLaneDispatch_PreflightRefusalNeverPlansNorLaunches",
		cmdEvolvePkg)
	requireTestsRan(t, out, 3)
	if code != 0 {
		t.Errorf("fleet min-width lane fallback is red (exit=%d) — forceOneLaneDispatch missing or wrong\n%s", code, out)
	}
}

func TestC547_002_ShouldRunWave_CountGateUnloosened(t *testing.T) {
	out, code := runGoTest(t, "TestShouldRunWave_CountOneOrZeroStillFalse", cmdEvolvePkg)
	requireTestsRan(t, out, 1)
	if code != 0 {
		t.Errorf("shouldRunWave's Count>1 gate regressed (exit=%d) — a fleet.count=1 static config must NOT be routed through the wave path\n%s", code, out)
	}
}

func TestC547_003_MemoOverlayRoutesWithoutWarning(t *testing.T) {
	out, code := runGoTest(t,
		"TestValidateUserSpecWithCatalog_ExemptsOptionalBuiltinName|TestValidateUserSpecWithCatalog_RejectsGenuineNewSingleWordName|TestValidateUserSpecWithCatalog_RejectsNonOptionalBuiltinNameOverlay|TestApplyUserRouting_RoutesBuiltinNameOverlayWithoutWarning",
		phasespecPkg)
	requireTestsRan(t, out, 4)
	if code != 0 {
		t.Errorf("memo-phase-routing-repair is red (exit=%d) — ValidateUserSpecWithCatalog/ApplyUserRouting missing or wrong\n%s", code, out)
	}
}

func TestC547_004_NewUngraduatedPackages_PureFn(t *testing.T) {
	out, code := runGoTest(t,
		"TestNewUngraduatedPackages_FlagsChangedInternalPkgAbsentFromEnforceList|TestNewUngraduatedPackages_AlreadyGraduatedPackagesNotFlagged|TestNewUngraduatedPackages_CmdPackagesNeverFlagged|TestNewUngraduatedPackages_DedupesAndSorts",
		ciparityPkg)
	requireTestsRan(t, out, 4)
	if code != 0 {
		t.Errorf("apicover new-package graduation pure function is red (exit=%d) — NewUngraduatedPackages missing or wrong\n%s", code, out)
	}
}

func TestC547_005_ApicoverNewPkgGraduationGate_WiredIntoAudit(t *testing.T) {
	out, code := runGoTest(t,
		"TestApicoverNewPkgGraduation_OffendersFailAudit|TestApicoverNewPkgGraduationDefault_NoUngraduatedPackages_NoOp|TestApicoverNewPkgGraduationDefault_UngraduatedPackageFlagged|TestApicoverNewPkgGraduationDefault_CmdChangeNotFlagged",
		auditPkg)
	requireTestsRan(t, out, 4)
	if code != 0 {
		t.Errorf("apicover new-package graduation gate is red or not wired into audit (exit=%d)\n%s", code, out)
	}
}
