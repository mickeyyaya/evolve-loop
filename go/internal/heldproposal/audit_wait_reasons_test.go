package heldproposal

import (
	"testing"
)

func TestWaitReasons_EachCodeIsDistinctAndRoundTripsThroughAWaitReport(t *testing.T) {
	t.Parallel()
	reasons := []string{
		WaitFloorExcludesOtherFamilies,
		WaitNoIndependentRoute,
		WaitBuilderUnattributed,
		WaitSeatFailed,
		WaitPortNotWired,
		WaitShipWindowHeld,
		WaitDiffMissing,
		WaitDiffMismatch,
	}
	seen := map[string]bool{}
	for _, reason := range reasons {
		if reason == "" || seen[reason] {
			t.Fatalf("wait reason %q is empty or repeated in %v", reason, reasons)
		}
		seen[reason] = true
	}
	dir := t.TempDir()
	for i, reason := range reasons {
		cycle := i + 1
		report := AuditReport{Cycle: cycle, Tree: "tree-a", Verdict: AuditWait, Reason: reason + ": detail"}
		if err := WriteAuditReport(dir, report, nil); err != nil {
			t.Fatal(err)
		}
		got, body, found, err := ReadAuditReport(dir, cycle)
		if err != nil || !found || got.Verdict != AuditWait || got.Reason != reason+": detail" || body != nil {
			t.Errorf("%s: read %+v body %v found %v err %v, want the WAIT report with its reason and no body", reason, got, body, found, err)
		}
	}
}
