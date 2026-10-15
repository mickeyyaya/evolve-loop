package phaseoutputs

import "testing"

func TestCoverfixSurveyNilResolverFallsBackToBuiltinContracts(t *testing.T) {
	t.Parallel()
	got := Survey([]string{"tdd"}, files("test-report.md", "tdd-engineer-prompt.txt"), nil)
	if len(got.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(got.Rows))
	}
	row := got.Rows[0]
	if row.Agent != "tdd-engineer" {
		t.Errorf("agent = %q, want the builtin contract's tdd-engineer", row.Agent)
	}
	if !row.Report.Present || row.Report.Name != "test-report.md" {
		t.Errorf("builtin contract must locate test-report.md, got %+v", row.Report)
	}
	if !row.Prompt.Present || row.Prompt.Name != "tdd-engineer-prompt.txt" {
		t.Errorf("prompt must resolve through the builtin agent name, got %+v", row.Prompt)
	}
}

func TestCoverfixSummaryLineOfEmptySurveyHasNothingToSurvey(t *testing.T) {
	t.Parallel()
	if got := (Result{}).SummaryLine(); got != "phase outputs: nothing to survey" {
		t.Errorf("SummaryLine = %q", got)
	}
}
