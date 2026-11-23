//go:build acs

// Package cycle2611 encodes the acceptance of the inbox closures of an integration batch: the next sync-main passes without a manual ship.
package cycle2611

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runJourney(t *testing.T, name string) string {
	t.Helper()
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-count=1", "-v", "-run", "^"+name+"$", "./cmd/evolve")
	if err != nil || code != 0 {
		t.Fatalf("%s failed: exit=%d err=%v\n%s\n%s", name, code, err, acsassert.OutputTail(out, 25), acsassert.OutputTail(errOut, 10))
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("%s did not run to a PASS:\n%s", name, acsassert.OutputTail(out, 25))
	}
	return out
}

func TestC2611_001_SyncMainPassesAfterABatchThatClosedAnInboxItem(t *testing.T) {
	runJourney(t, "TestJourneyInboxClosureBatchThenSyncMainPasses")
}

func TestC2611_002_SyncMainStillRefusesATrackedDeletionThatIsNoClosure(t *testing.T) {
	runJourney(t, "TestJourneyInboxClosureStillRefusesOtherTrackedDeletions")
}

func TestC2611_003_ABatchThatClosedNothingLeavesSyncMainUnchanged(t *testing.T) {
	runJourney(t, "TestJourneyInboxClosureWithNoClosedItemLeavesSyncMainUnchanged")
}
