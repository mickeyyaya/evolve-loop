//go:build acs

package cycle2597

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func syncMainTests(t *testing.T, names ...string) bool {
	t.Helper()
	pattern := "^("
	for i, name := range names {
		if i > 0 {
			pattern += "|"
		}
		pattern += name
	}
	pattern += ")$"
	return acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     acsassert.GoModuleDir(t),
		Package: "./cmd/evolve",
		Pattern: pattern,
		Names:   names,
	})
}

func TestC2597_001_ConsoleRouteEditRidesThroughTheMerge(t *testing.T) {
	syncMainTests(t, "TestSyncMain_CarriesAConsoleRouteEditThroughTheMerge")
}

func TestC2597_002_ConsoleConsumeMoveRidesThroughTheMerge(t *testing.T) {
	syncMainTests(t,
		"TestSyncMain_CarriesAConsoleConsumeMove",
		"TestSyncMain_AConsumeMoveThatAlsoRewritesAnAuthoredFieldStillRefuses",
	)
}

func TestC2597_003_UntrackedCopyIdenticalToOriginIsRemovedAndDifferingKept(t *testing.T) {
	syncMainTests(t, "TestSyncMain_RemovesUntrackedCopiesIdenticalToOrigin")
}

func TestC2597_004_SameFieldConflictKeepsOriginAndNamesTheDroppedEdit(t *testing.T) {
	syncMainTests(t, "TestSyncMain_ASameFieldConflictKeepsOriginAndNamesTheDroppedConsoleEdit")
}

func TestC2597_005_ParallelDefaultGateAcceptsTheSyncMainTestFiles(t *testing.T) {
	syncMainTests(t, "TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial")
}
