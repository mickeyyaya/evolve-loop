//go:build acs

package cycle2386

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runShipTests(t *testing.T, pattern string, names ...string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", goDir, "test", "-count=1", "-run", pattern, "-v", "./internal/phases/ship")
	if err != nil || code != 0 {
		t.Errorf("`-run %s ./internal/phases/ship` is not green (exit=%d err=%v)\n%s\n%s",
			pattern, code, err, acsassert.OutputTail(stdout, 40), acsassert.OutputTail(stderr, 40))
	}
	for _, name := range names {
		if !strings.Contains(stdout, "=== RUN   "+name) {
			t.Errorf("`-run %s` never selected %s", pattern, name)
		}
		if !strings.Contains(stdout, "--- PASS: "+name) {
			t.Errorf("%s did not pass", name)
		}
	}
}

func TestC2386_001_LaneNamingAnotherLanesItemShipsOwnAndLeavesTheOtherInPlace(t *testing.T) {
	name := "TestPromoteInbox_LaneNamingAnotherLanesItemShipsOwnAndLeavesTheOtherInPlace"
	runShipTests(t, name, name)
}

func TestC2386_002_RefusalWarnNamesEachRefusedID(t *testing.T) {
	name := "TestPromoteInbox_RefusalWarnNamesEachRefusedID"
	runShipTests(t, name, name)
}

func TestC2386_003_CycleWithoutLanePinCannotRetireAnotherCyclesHeldItem(t *testing.T) {
	name := "TestPromoteInbox_CycleWithoutLanePinCannotRetireAnotherCyclesHeldItem"
	runShipTests(t, name, name)
}

func TestC2386_004_LanePinRefusesAnUnheldInboxItemOutsideThePin(t *testing.T) {
	name := "TestPromoteInbox_LanePinRefusesAnUnheldInboxItemOutsideThePin"
	runShipTests(t, name, name)
}

func TestC2386_005_LegitimateLanePromotionIsPreserved(t *testing.T) {
	runShipTests(t,
		"TestPromoteInbox_(LaneScopeFallbackPromotesOnPass|UnlandedShipSkipsLaneFallbackPromotion|EmptyCommittedDeclinedMenuStaysOpen|PromoteError_StillDrainsResidualClaims|LanePromotesItsPinnedAndHeldItemsWithoutRefusals)$",
		"TestPromoteInbox_LaneScopeFallbackPromotesOnPass",
		"TestPromoteInbox_EmptyCommittedDeclinedMenuStaysOpen",
		"TestPromoteInbox_LanePromotesItsPinnedAndHeldItemsWithoutRefusals",
	)
}
