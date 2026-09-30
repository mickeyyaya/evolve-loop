//go:build acs

package cycle1440

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	shipPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

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

func TestC1440_003_SecondIdenticalStageRefusalIsDeterministic(t *testing.T) {
	runNamedTests(t, shipPkg, "^TestStageRefusal_", []string{
		"TestStageRefusal_FirstStrikeStaysTransient",
		"TestStageRefusal_SecondSamePathspecIsDeterministic",
		"TestStageRefusal_DifferentPathspecStaysTransient",
		"TestStageRefusal_SeparateWorkspacesDoNotShareStrikes",
		"TestStageRefusal_NoWorkspaceStaysTransient",
	})
}

func TestC1440_004_FingerprintFoldsPathAndAttemptVariance(t *testing.T) {
	runNamedTests(t, corePkg, "^TestNormalizeReasonForFingerprint_", []string{
		"TestNormalizeReasonForFingerprint_CycleNumberedPathsFold",
		"TestNormalizeReasonForFingerprint_AttemptDenominatorFolds",
		"TestNormalizeReasonForFingerprint_DistinctDefectsStayDistinct",
		"TestNormalizeReasonForFingerprint_ExistingPinsStayGreen",
		"TestNormalizeReasonForFingerprint_TouchesOnlyTheNarrativeToken",
	})
}
