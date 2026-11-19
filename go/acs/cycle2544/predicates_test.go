//go:build acs

// Package cycle2544 holds the predicates of the scope-delta floor summary contract.
package cycle2544

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func assertCoreTestsPass(t *testing.T, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", pattern, "-v", "-count=1", corePkg)
	if code == -1 {
		t.Fatalf("go test failed to launch for %s: %v\n%s", corePkg, err, stderr)
	}
	out := stdout + stderr
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not pass in %s (go test exit %d)", name, corePkg, code)
		}
	}
	if code != 0 {
		t.Errorf("RED: go test exited %d for %s:\n%s", code, corePkg, tail(out, 2000))
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func TestC2544_001_ThreeOutOfScopePathsGiveOneSummarySignalAndAThreePathArtifact(t *testing.T) {
	assertCoreTestsPass(t,
		"TestScopeDeltaFloorChecks_ReportsOneSummarySignalForAllOutOfScopePaths",
		"TestScopeDeltaFloorChecks_ASummaryOfOnlyKeptPathsIsInfo",
		"TestScopeDeltaFloorChecks_AnUnreadablePinLosesTheWaveNotTheReport",
		"TestScopeDeltaFloorChecks_IsSilentWithoutAScopeToCompareAgainst",
	)
}

func TestC2544_002_ASecondPassOverTheSameTreeEmitsNoNewSignal(t *testing.T) {
	assertCoreTestsPass(t,
		"TestScopeDeltaFloorChecks_ASecondPassOverTheSameTreeEmitsNoNewSignal",
		"TestScopeDeltaFloorChecks_ASameSetInAnotherWorkspaceIsReportedAgain",
	)
}

func TestC2544_003_APassWhoseSetChangedEmitsOneNewSummarySignal(t *testing.T) {
	assertCoreTestsPass(t,
		"TestScopeDeltaFloorChecks_APassWhoseSetChangedEmitsOneNewSummary",
		"TestScopeDeltaFloorChecks_AnInScopeOnlyTreeEmitsNoSignalAndNoArtifact",
	)
}
