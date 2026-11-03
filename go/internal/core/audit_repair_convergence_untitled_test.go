package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
)

func TestAuditRepairConvergence_InputKeysAnUntitledFindingByItsReportId(t *testing.T) {
	t.Parallel()
	untitled := "# Audit\n\n## Verdict\nFAIL\n\n## Issues\n| H7 | HIGH |  |\n"

	got := openAuditFindings(untitled)

	f, ok := got["H7"]
	if !ok || len(got) != 1 {
		t.Fatalf("an untitled finding is keyed by its report id H7, got %+v", got)
	}
	if f.Severity != convergence.SeverityHigh || f.Status != convergence.StatusOpen {
		t.Errorf("finding = %+v, want an OPEN HIGH", f)
	}
	rounds := judgmentsOf([]string{untitled})
	in := auditRepairConvergenceInput(convergenceWorkflow("shadow", 3).Convergence, rounds, convergence.TierEffort{}, convergence.TierEffort{}, nil)
	if err := in.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}
