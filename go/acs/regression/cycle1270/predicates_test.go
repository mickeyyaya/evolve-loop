//go:build acs

package cycle1270

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	gitexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	bridgePkg  = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	retroPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func runNamedTests(t *testing.T, pkg string, names ...string) string {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	return stdout + stderr
}

func assertNamedTestsPass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", pkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\nstderr:\n%s", pkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("test %s did NOT pass in %s "+
				"(missing, failing, or hidden behind a build tag the default suite skips).\n"+
				"combined go-test output:\n%s", name, pkg, out)
		}
	}
}

func TestC1270_001_PermanentWorktreeAddFailureCostsZeroBackoff(t *testing.T) {
	assertNamedTestsPass(t, gitexecPkg,
		"TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff",
		"TestAddWorktreeWithRetry_TransientStillRetriesToBound",
		"TestAddWorktreeWithRetry_NilRetryablePreservesRetryEverything",
		"TestAddWorktreeWithRetry_RetriesTransientFailure",
		"TestAddWorktreeWithRetry_BoundedThenSurfacesFinalFailure",
		"TestAddWorktreeWithRetry_CleanRunCostsOneAttemptAndNoSleep",
		"TestAddWorktreeWithRetry_ZeroValueConfigUsesDefaults",
		"TestAddWorktreeWithRetry_IssuesWorktreeAddArgv",
	)
}

func TestC1270_002_CoreSuppliesTransiencePredicateAndHonestAnnouncement(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestWorktreeAddRetry_NotAGitRepositoryCostsZeroBackoff",
		"TestWorktreeAddRetry_LockCollisionStillRetriesToBound",
		"TestWorktreeAddRetry_AnnouncementDoesNotClaimUnclassifiedTransience",
		"TestBuildFloorSelfCheckFailures_KeepsTailDiagnostic",
		"TestGitWorktreeCreate_PersistentFailureStillFailsLoudly",
		"TestGitWorktreeCreate_RetriesTransientAddFailure",
	)
}

func TestC1270_003_CmdEvolveNoLongerPaysTheRetryLadder(t *testing.T) {
	const name = "TestLoop_MaxCyclesExit_ClearsCompletedMarker"
	out := runNamedTests(t, cmdPkg, name)
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("%s did not pass in %s — the blocker fix must not break loop provisioning.\n"+
			"combined go-test output:\n%s", name, cmdPkg, out)
	}
	if strings.Contains(out, "[worktree] retry ") {
		t.Errorf("the worktree retry ladder still fires under %s.\n"+
			"The failure it retries is PERMANENT (`fatal: not a git repository` on a t.TempDir()), "+
			"so every announcement is 2s+4s of pure sleep bought for nothing — 33 tests x 6s = 198s "+
			"of this package's wall time. The build floor runs every package under addedtests.PackageTimeout, "+
			"so a slow package spends its wall-time budget instead of tripping a deadline "+
			"(inbox cmd-evolve-unit-wall-time-and-package-budget-check). Classify the failure before "+
			"sleeping (gitexec/worktree.go:59-70) rather than paying for it in sleep.\n"+
			"combined go-test output:\n%s", name, out)
	}
}

func TestC1270_004_UnionUniquenessScanCoversConsumerPackages(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestInfraTeardownUnion_ScanCoversConsumerPackages",
		"TestInfraTeardownUnion_DetectsPlantedDuplicateOutsideCore",
		"TestInfraTeardownUnion_SpelledExactlyOnce",
	)
}

func TestC1270_005_NarrowerPredicateSitesSurviveTheWidening(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestTimeoutOnlySites_NotWidenedToUnion",
		"TestWritePhaseFailureDiag_TimeoutOnlyNotWidened",
		"TestIsTransientBridgeError_StaysTransientOnly",
		"TestOptionalInfraSkip_GateAgreesWithIsOptionalSkippableError",
	)
}

func TestC1270_006_MintedScratchCwdClearsTheFleetGuard(t *testing.T) {
	assertNamedTestsPass(t, bridgePkg,
		"TestFleetModeAcceptsScratchCwdWorktree",
		"TestFleetModeRefusesEmptyWorktree",
	)
}

func TestC1270_007_RetroFleetDispatchCarriesLaneWorktreeEndToEnd(t *testing.T) {
	assertNamedTestsPass(t, retroPkg,
		"TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate",
		"TestRetro_EmptyWorktree_FallsBackToScratchUnderWorkspace",
		"TestRetro_EmptyWorktree_NeverMainTreeOrProcessCwd",
		"TestRetro_RealWorktree_PassedThroughUnchanged",
		"TestRetro_EmptyWorktreeAndWorkspace_NoFabricatedPath",
	)
}

func TestC1270_008_AbsentCoveringCorpusIsNeverSilent(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestCoveringTests_AbsentCorpusIsAnnouncedNotSilent",
		"TestCoveringTests_AvailableToTestAmplificationWithoutFreshBuild",
	)
}

func TestC1270_009_CoveringCorpusContractSurvives(t *testing.T) {
	assertNamedTestsPass(t, corePkg,
		"TestWriteCoveringTests_NoOpWithoutWorktreeOrWorkspace",
		"TestRenderCoveringTests_ReportsOmittedCount",
		"TestWriteCoveringTests_WarnsLoudlyOnTruncation",
		"TestWriteCoveringTests_SilentWhenNothingTruncated",
		"TestRenderCoveringTests_NeutralizesInjectedMarkdown",
		"TestRenderCoveringTests_LeavesBenignPathsVerbatim",
	)
}
