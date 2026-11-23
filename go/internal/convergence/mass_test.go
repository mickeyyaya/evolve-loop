package convergence

import "testing"

func TestFindingMass_UsesTheDefaultWeights(t *testing.T) {
	t.Parallel()

	got := FindingMass([]Finding{{Severity: SeverityCritical}, {Severity: SeverityHigh}, {Severity: SeverityMedium}, {Severity: SeverityLow}, {Severity: SeverityInfo}})
	if got != 15 {
		t.Fatalf("mass = %v, want 8+4+2+1+0 = 15", got)
	}
}

func TestMassAt_IsTheMassDecideUsesAtTheBarWithoutRuleFiledFindings(t *testing.T) {
	t.Parallel()

	in := Input{Loop: LoopCodeReview, Rounds: []Judgment{{Findings: []Finding{
		{ID: "a", Severity: SeverityHigh, Status: StatusOpen},
		{ID: "b", Severity: SeverityMedium, Status: StatusOpen},
		{ID: "c", Severity: SeverityHigh, Status: StatusFixed},
	}}}}
	if got := MassAt(in, 0, SeverityHigh); got != 4 {
		t.Fatalf("mass at HIGH = %v, want only the OPEN HIGH row", got)
	}
	if got := MassAt(in, 0, SeverityMedium); got != 6 {
		t.Fatalf("mass at MEDIUM = %v, want 4+2", got)
	}
	in.Mass = func(_ int, atBar []Finding) float64 { return float64(len(atBar)) * 100 }
	if got := MassAt(in, 0, SeverityMedium); got != 200 {
		t.Fatalf("mass with a Mass hook = %v, want the hook over the 2 strict findings", got)
	}
}
