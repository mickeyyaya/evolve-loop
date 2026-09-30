//go:build acs

package cycle1268

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	gitexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	corePkg    = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	swarmPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	cmdPkg     = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
)

func assertDefaultSuiteTestsPass(t *testing.T, pkg string, names ...string) {
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
				"(missing, failing, or hidden behind a build tag the default suite skips). exit=%d\n"+
				"combined go-test output:\n%s", name, pkg, code, out)
		}
	}
}

func TestC1268_001_SharedWorktreeAddRetryHelperExists(t *testing.T) {
	assertDefaultSuiteTestsPass(t, gitexecPkg,
		"TestAddWorktreeWithRetry_RetriesTransientFailure",
		"TestAddWorktreeWithRetry_BoundedThenSurfacesFinalFailure",
		"TestAddWorktreeWithRetry_CleanRunCostsOneAttemptAndNoSleep",
		"TestAddWorktreeWithRetry_ZeroValueConfigUsesDefaults",
		"TestAddWorktreeWithRetry_IssuesWorktreeAddArgv",
	)
}

func TestC1268_002_CreateFromRetriesTransientCollision(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestGitWorktreeCreateFrom_RetriesTransientAddFailure",
		"TestGitWorktreeCreateFrom_PersistentFailureStillFailsLoudly",
		"TestGitWorktreeCreateFrom_CleanRunCostsOneAttemptAndNoSleep",
		"TestGitWorktreeCreate_RetriesTransientAddFailure",
		"TestGitWorktreeCreate_PersistentFailureStillFailsLoudly",
	)
}

func TestC1268_003_SwarmWorkerProvisioningRetries(t *testing.T) {
	assertDefaultSuiteTestsPass(t, swarmPkg,
		"TestSwarmCreateWorker_RetriesTransientAddFailure",
		"TestSwarmCreateIntegration_RetriesTransientAddFailure",
		"TestSwarmCreateWorker_PersistentFailureStillFailsLoudly",
		"TestSwarmCreateWorker_CleanRunCostsOneAttemptAndNoSleep",
	)
}

func TestC1268_004_OperatorCLIWorktreeCreateRetries(t *testing.T) {
	assertDefaultSuiteTestsPass(t, cmdPkg,
		"TestRunWorktreeCreate_RetriesTransientAddFailure",
		"TestRunWorktreeCreate_PersistentFailureStillFailsLoudly",
		"TestRunWorktreeCreate_CleanRunCostsOneAttemptAndNoSleep",
	)
}

func TestC1268_005_CoveringTestsCorpusResistsPromptInjection(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestRenderCoveringTests_NeutralizesInjectedMarkdown",
		"TestRenderCoveringTests_LeavesBenignPathsVerbatim",
	)
}

func TestC1268_006_CoveringTestsDerivationStaysGreen(t *testing.T) {
	assertDefaultSuiteTestsPass(t, "github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs",
		"TestCoveringTests_DerivesTestFilesForChangedPackagesOnly",
		"TestCoveringTests_FailsOpenOnUnusableInput",
		"TestCoveringTests_ReachableFromProduction",
		"TestDirectImporters_WidensToReverseImportersIncludingTestOnly",
		"TestDirectImporters_FailsOpenOnUnusableInput",
		"TestDirectImporters_ReachableFromProduction",
	)
}

func TestC1268_007_CoveringTestsCapStaysLoudAndCorrect(t *testing.T) {
	assertDefaultSuiteTestsPass(t, corePkg,
		"TestRenderCoveringTests_TruncatesWithVisibleNote",
		"TestRenderCoveringTests_EmitsEveryPathWhenUnderCap",
		"TestRenderCoveringTests_ReportsOmittedCount",
		"TestWriteCoveringTests_WarnsLoudlyOnTruncation",
		"TestWriteCoveringTests_SilentWhenNothingTruncated",
	)
}
