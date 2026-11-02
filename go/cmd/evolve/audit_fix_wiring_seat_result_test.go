package main

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfixports"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

func TestSeatResultOf_CarriesTheVerdictFieldsTheReviewerReads(t *testing.T) {
	t.Parallel()
	env := agentcmd.Envelope{
		RunLabel: "agent-code-reviewer-1", Family: "agy", Verdict: "FAIL",
		Findings: reportdoc.SeverityCounts{Critical: 1, High: 2, Medium: 3}, Error: "no launch",
	}
	want := auditfixports.SeatResult{RunLabel: "agent-code-reviewer-1", Family: "agy", Verdict: "FAIL", Critical: 1, High: 2, Error: "no launch"}
	if got := seatResultOf(env); got != want {
		t.Fatalf("seatResultOf = %+v, want %+v", got, want)
	}
}
