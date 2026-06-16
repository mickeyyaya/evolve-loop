package routingtest

import (
	"testing"
)

// TestFullOrchestrator_SelectsCycleSurface names the FullOrchestrator surface
// const and pins its contract: a ScenarioSpec carrying Surface=FullOrchestrator
// is executed end-to-end (Run drives the full mandatory spine), unlike
// PureKernel which would make a single Route() decision.
func TestFullOrchestrator_SelectsCycleSurface(t *testing.T) {
	spec := Scenario("apicover-fullorch",
		Cycle(), // sets Surface = FullOrchestrator
		// Default static spine for this surface, dispatched end-to-end.
		ExpectPhases("scout", "triage", "tdd", "build-planner", "build", "audit", "ship"),
	)
	if spec.Surface != FullOrchestrator {
		t.Fatalf("Surface = %d, want FullOrchestrator (%d)", spec.Surface, FullOrchestrator)
	}
	// Run drives the orchestrator surface and self-asserts the phase sequence.
	Run(t, spec)
}

// TestDimension_ExpandsInMatrix names the Dimension type and pins that Dim()
// builds one and Matrix expands base × dimension into one spec per variant: a
// single 2-variant Dimension on a Pure base yields exactly 2 labeled scenarios.
func TestDimension_ExpandsInMatrix(t *testing.T) {
	var dim Dimension = Dim("size", V("trivial", TrivialCycle()), V("medium", MediumCycle()))

	specs := Matrix([]Brick{Pure()}, dim)
	if len(specs) != 2 {
		t.Fatalf("Matrix expanded to %d specs, want 2 (one per variant)", len(specs))
	}
	wantNames := map[string]bool{"size=trivial": true, "size=medium": true}
	for _, s := range specs {
		if !wantNames[s.Name] {
			t.Errorf("unexpected scenario name %q; want one of %v", s.Name, wantNames)
		}
		if s.Surface != PureKernel {
			t.Errorf("scenario %q Surface = %d, want PureKernel (base brick)", s.Name, s.Surface)
		}
	}
}

// TestFailedRecordSpec_SeedsFailedApproaches names the FailedRecordSpec type and
// pins its role: each spec seeds one failedApproaches entry that failedRecords
// renders into the orchestrator's retro arc input, threading Classification
// through unchanged and stamping a non-expired RecordedAt.
func TestFailedRecordSpec_SeedsFailedApproaches(t *testing.T) {
	specs := []FailedRecordSpec{
		{Classification: "flaky-test", Verdict: "FAIL"},
		{Classification: "lint-drift", Verdict: "WARN"},
	}
	recs := failedRecords(specs)
	if len(recs) != len(specs) {
		t.Fatalf("failedRecords produced %d records, want %d", len(recs), len(specs))
	}
	if recs[0].Classification != "flaky-test" {
		t.Errorf("record[0].Classification = %q, want flaky-test", recs[0].Classification)
	}
	if recs[0].RecordedAt != nonExpiredRecordedAt {
		t.Errorf("record[0].RecordedAt = %q, want non-expired %q", recs[0].RecordedAt, nonExpiredRecordedAt)
	}
}
