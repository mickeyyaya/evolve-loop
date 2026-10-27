package selfheal

import (
	"slices"
	"testing"
)

func TestClasses_IsTheClosedSetOfTheDetectionRules(t *testing.T) {
	t.Parallel()
	want := []Class{ClassMainRed, ClassCIHang, ClassFlakyTest, ClassQuality, ClassIsolation, ClassStall, ClassSystem, ClassZeroShip, ClassLaneStarvation, ClassRepairLoop, ClassClaimChurn, ClassUsageBlind}
	if !slices.Equal(Classes(), want) {
		t.Fatalf("Classes() = %v", Classes())
	}
}

func TestTreeVerifier_IsTheVerifierOfThePass(t *testing.T) {
	t.Parallel()
	var v Verifier = TreeVerifier{}
	if v.ReproGreen(Incident{}) || v.WaveEndedAfter(t0) {
		t.Fatal("an empty tree proves no condition")
	}
}

func TestFromCycles_OneRepairRoundUnderTheLimitIsNoLoop(t *testing.T) {
	t.Parallel()
	if got := FromCycles([]Cycle{{ID: 1, Item: "a", RepairAttempts: RepairLoopAfter - 1, Started: t0}}); len(got) != 0 {
		t.Fatalf("%d repair round is under the limit of %d: %+v", RepairLoopAfter-1, RepairLoopAfter, got)
	}
}
