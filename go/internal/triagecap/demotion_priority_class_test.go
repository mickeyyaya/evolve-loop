package triagecap

import "testing"

func TestNewDemotionLedgerRecord_IsCorrectnessClassed(t *testing.T) {
	rec := NewDemotionLedgerRecord(303, 301, 302, "detail", RemedyPending)
	if rec.PriorityClass != "correctness" {
		t.Errorf("a gate suspected of a false verdict is a correctness defect; priority_class = %q", rec.PriorityClass)
	}
}
