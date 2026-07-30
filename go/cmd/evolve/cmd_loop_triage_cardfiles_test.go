package main

// cmd_loop_triage_cardfiles_test.go — the WRITER→CONSUMER proof for
// triage-cards-carry-files. The two halves are individually tested
// (internal/triagecap/cardfiles_test.go for the declaration, PR #366's menu tests
// for the partitioning); what failed live on batch-14 was the JOIN: the
// orchestrator's projection dropped the footprint, so by the time the next wave
// planned, the card's file knowledge existed only in prose and the planner saw an
// id island.
//
// This drives the REAL production chain end to end:
//
//	triage-report.md  →  triagecap.ProjectDecisionJSON   (ship/postship writes it)
//	                  →  widenNarrowDecision             (next wave's primary path)
//	                  →  fleet.PlanFromTriage            (lane partitioning)
//
// RED before the projection carried files=: "mate" cannot join the committed
// card's lane, because nothing downstream knows they touch the same file.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// cardFilesTriageReport is the shape triage writes: the declared footprint rides
// the item's metadata tail, exactly as agents/evolve-triage.md now requires.
const cardFilesTriageReport = `<!-- challenge-token: abc -->
<!-- ANCHOR:triage_decision -->
# Triage Decision — Cycle 1167

cycle_size_estimate: small
phase_skip: []

## top_n (commit to THIS cycle)
- committed: reconcile the auditor verdict with EGPS — priority=H, files=go/internal/x/a.go, source=scout

## Rationale
One audit-surface item this cycle.
`

// TestProjectedDecisionFilesReachTheLanePlanner is the join: a footprint declared
// in the report must survive projection and make the fleet planner cluster the
// same-file backlog mate into the SAME lane instead of a fictional-disjoint
// second lane (the 948 lost-work class).
func TestProjectedDecisionFilesReachTheLanePlanner(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItemFiles(t, inbox, "mate.json", "mate", 0.70, "go/internal/x/a.go")
	writeInboxItemFiles(t, inbox, "other.json", "other", 0.80, "go/internal/y/b.go")

	projected, err := triagecap.ProjectDecisionJSON(cardFilesTriageReport, 1167)
	if err != nil {
		t.Fatalf("ProjectDecisionJSON: %v", err)
	}
	specs, _, err := fleet.PlanFromTriage(widenNarrowDecision(projected, dir, 2), nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage over the projected decision: %v", err)
	}

	byLane := map[string]int{}
	for lane, s := range specs {
		for _, id := range s.Scope {
			byLane[id] = lane
		}
	}
	committedLane, ok := byLane["committed"]
	if !ok {
		t.Fatalf("the committed card vanished from the plan: %+v", specs)
	}
	mateLane, ok := byLane["mate"]
	if !ok {
		t.Fatalf("the same-file backlog mate never entered the plan: %+v", specs)
	}
	if mateLane != committedLane {
		t.Errorf("mate (lane %d) and committed (lane %d) both touch go/internal/x/a.go but were planned "+
			"into DIFFERENT concurrent lanes — the projected card carried no files[], so the planner saw "+
			"an id island and two lanes will edit one file", mateLane, committedLane)
	}
	if other, ok := byLane["other"]; !ok {
		t.Errorf("the genuinely disjoint item must still widen the wave to a second lane: %+v", specs)
	} else if other == committedLane {
		t.Errorf("the disjoint item collapsed into the committed lane (%d) — clustering must not cost width", other)
	}
}
