package routingtest

import (
	"testing"
)

func TestFullOrchestrator_SelectsCycleSurface(t *testing.T) {
	spec := Scenario("apicover-fullorch",
		Cycle(),
		ExpectPhases("scout", "triage", "tdd", "build-planner", "build", "audit", "ship"),
	)
	if spec.Surface != FullOrchestrator {
		t.Fatalf("Surface = %d, want FullOrchestrator (%d)", spec.Surface, FullOrchestrator)
	}
	Run(t, spec)
}

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
