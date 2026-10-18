//go:build acs

package acsnames

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var absentOnPurpose = map[string]string{
	"cycle1067/TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll":      "the predicate runs the name to prove that the superseded test is gone",
	"cycle1712/TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation": "the predicate asserts that the test was migrated away",
	"cycle1718/TestLoopUnblockProfilesRouteTimeoutPronePhasesToAgy":       "the predicate asserts that the test was renamed away",
	"cycle1737/TestC1737ProbeNoCycleBatchEndSink":                         "the predicate builds this probe test into an overlay by concatenation and runs it",
	"cycle1765/TestC1761_":                          "the predicate asserts that the eval no longer cites this prefix",
	"cycle266/TestInvariant_DuplicatePhaseRejected": "the predicate asserts that the test was renamed away",
}

func TestPredicateNamesResolveToTests(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	violations, err := findViolations(goDir, absentOnPurpose)
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Error(violation)
	}
}

func TestAbsentOnPurposeEntriesCarryAReason(t *testing.T) {
	for key, reason := range absentOnPurpose {
		if reason == "" {
			t.Errorf("absent list entry %q has no reason", key)
		}
	}
}
