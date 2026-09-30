//go:build acs

package cycle959

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const cmdEvolvePkg = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"

func runGoTest(t *testing.T, pkg, runExpr string, wantPass []string) {
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

func TestC959_001_PoolHaltCodeLaneStopsBatch(t *testing.T) {
	runGoTest(t, cmdEvolvePkg,
		"^TestDispatchHaltDecision_HaltsOnSystemFailureLane$",
		[]string{"TestDispatchHaltDecision_HaltsOnSystemFailureLane"})
}

func TestC959_002_OrdinaryAndEmptyFailuresContinue(t *testing.T) {
	runGoTest(t, cmdEvolvePkg,
		"^TestDispatchHaltDecision_(OrdinaryFailuresContinue|EmptyResultsContinue)$",
		[]string{
			"TestDispatchHaltDecision_OrdinaryFailuresContinue",
			"TestDispatchHaltDecision_EmptyResultsContinue",
		})
}

func TestC959_003_WavePathHaltTestsStillGreen(t *testing.T) {
	runGoTest(t, cmdEvolvePkg,
		"^Test(AnyLaneHaltedForSystemFailure|CycleRunExitCode)",
		[]string{
			"TestAnyLaneHaltedForSystemFailure_DetectsHaltExitCodeAmongLanes",
			"TestAnyLaneHaltedForSystemFailure_OrdinaryLaneFailuresDoNotHalt",
			"TestCycleRunExitCode_HaltsOnSystemFailureRegardlessOfVerdict",
		})
}

func TestC959_004_CmdEvolveBuilds(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "build", cmdEvolvePkg)
	if code != 0 || err != nil {
		t.Fatalf("go build %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			cmdEvolvePkg, code, err, stdout, stderr)
	}
}

func TestC959_005_CmdEvolveVetClean(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", cmdEvolvePkg)
	if code != 0 || err != nil {
		t.Fatalf("go vet %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s",
			cmdEvolvePkg, code, err, stdout, stderr)
	}
}
