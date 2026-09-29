//go:build acs

package cycle1754

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sizeratchet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func moduleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func cmdEvolveFile(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "cmd", "evolve", name)
}

func spanLines(t *testing.T, key string) int {
	t.Helper()
	spans, err := sizeratchet.Walk(moduleRoot(t))
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	lines := -1
	for _, s := range spans {
		if s.Key == key && s.Lines > lines {
			lines = s.Lines
		}
	}
	if lines < 0 {
		t.Fatalf("%s not found: the function must keep its name and package, a rename is not a shrink", key)
	}
	return lines
}

func countInFunc(t *testing.T, file, fn, needle string) int {
	t.Helper()
	n, err := acsassert.CountInGoFunc(cmdEvolveFile(t, file), fn, needle)
	if err != nil {
		t.Fatalf("CountInGoFunc(%s, %s): %v", file, fn, err)
	}
	return n
}

func TestC1754_001_RunResetSHAWithinSizeRatchetLimit(t *testing.T) {
	if lines := spanLines(t, "cmd/evolve.runResetSHA"); lines > sizeratchet.MaxLines {
		t.Errorf("RED: cmd/evolve.runResetSHA is %d lines > %d: extract its project-root resolution into a helper", lines, sizeratchet.MaxLines)
	}
}

func TestC1754_002_SetupLatestReportWithinSizeRatchetLimit(t *testing.T) {
	if lines := spanLines(t, "cmd/evolve.setupLatestReport"); lines > sizeratchet.MaxLines {
		t.Errorf("RED: cmd/evolve.setupLatestReport is %d lines > %d: extract its per-CLI goroutine body into probeCLILatest", lines, sizeratchet.MaxLines)
	}
}

func TestC1754_003_ModuleSizeRatchetCheckPasses(t *testing.T) {
	root := moduleRoot(t)
	offenders, err := sizeratchet.LoadOffenders(filepath.Join(root, "internal", "sizeratchet", "offenders.json"))
	if err != nil {
		t.Fatalf("LoadOffenders: %v", err)
	}
	spans, err := sizeratchet.Walk(root)
	if err != nil {
		t.Fatalf("sizeratchet.Walk: %v", err)
	}
	if err := sizeratchet.Check(spans, offenders); err != nil {
		t.Errorf("RED: the module-wide size ratchet must pass after both extractions (no helper past 50 lines, no listed function past its allowance): %v", err)
	}
}

func TestC1754_004_ResetSHARootResolutionBehaviorPreserved(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestRunResetSHA_",
		Names: []string{
			"TestRunResetSHA_OperatorRepinsToRunningBinary",
			"TestRunResetSHA_RefusesWithoutProvenanceOrOperator",
			"TestRunResetSHA_RootResolutionPrecedence",
			"TestRunResetSHA_RelativeRootResolvedAgainstCwd",
			"TestRunResetSHA_RelativeEnvRootResolvedAgainstCwd",
			"TestRunResetSHA_MissingRelativeRootFailsOnAbsoluteStatePath",
		},
	})
	for _, inline := range []string{"os.Getenv(", "os.Getwd(", "paths.AbsoluteRoot("} {
		if n := countInFunc(t, "cmd_resetsha.go", "runResetSHA", inline); n != 0 {
			t.Errorf("RED: runResetSHA still resolves its project root inline (%s x%d): the resolution block must move into a helper", inline, n)
		}
	}
}

func TestC1754_005_ProbeCLILatestContractAndCallerProof(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestProbeCLILatest_",
		Names: []string{
			"TestProbeCLILatest_StaleCurrentMarksTierStale",
			"TestProbeCLILatest_ListerErrorSkipsTierComputation",
			"TestProbeCLILatest_EmptyCandidatesNeverStale",
			"TestProbeCLILatest_CatalogTierOverridesManifest",
			"TestProbeCLILatest_UsesTheCLIsOwnFreshnessPolicy",
		},
	})
	if n := countInFunc(t, "cmd_setup_latest.go", "setupLatestReport", "probeCLILatest("); n == 0 {
		t.Errorf("RED: setupLatestReport never calls probeCLILatest: the extracted probe must be reached from its production caller, not duplicated beside it")
	}
	if n := countInFunc(t, "cmd_setup_latest.go", "setupLatestReport", "lister.List("); n != 0 {
		t.Errorf("RED: setupLatestReport still calls lister.List inline (x%d): the per-CLI probe body must live only in probeCLILatest", n)
	}
}

func TestC1754_006_SetupLatestReportFanOutBehaviorPreserved(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     moduleRoot(t),
		Package: cmdEvolvePkg,
		Pattern: "^TestSetupLatestReport_",
		Names: []string{
			"TestSetupLatestReport_QueriesEveryReadyFamilyInParallel",
			"TestSetupLatestReport_ProbeFailureIsOneRowNotTheReport",
			"TestSetupLatestReport_CatalogOverridesManifestBaseline",
			"TestSetupLatestReport_BalancedTierStalenessCounts",
			"TestSetupLatestReport_NeverSeenCurrentIsUnverifiedNotFresh",
			"TestSetupLatestReport_HungCaptureIsBoundedByTheTimeout",
			"TestSetupLatestReport_VanishedBalancedTierIsNamedUnverified",
		},
	})
}

func TestC1754_007_TouchedFilesGofmtAndVetClean(t *testing.T) {
	root := moduleRoot(t)
	for _, name := range []string{"cmd_resetsha.go", "cmd_resetsha_test.go", "cmd_setup_latest.go", "cmd_setup_latest_test.go"} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("gofmt", "-l", cmdEvolveFile(t, name))
		if err != nil || code != 0 {
			t.Fatalf("gofmt -l %s: exit=%d err=%v stderr=%s", name, code, err, stderr)
		}
		if stdout != "" {
			t.Errorf("RED: gofmt reports %s unformatted", name)
		}
	}
	_, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", root, "./cmd/evolve/")
	if err != nil || code != 0 {
		t.Errorf("RED: go vet ./cmd/evolve/ failed: exit=%d err=%v\n%s", code, err, stderr)
	}
}
