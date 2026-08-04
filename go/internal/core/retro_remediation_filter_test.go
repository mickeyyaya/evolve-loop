package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// retro_remediation_filter_test.go — RED contract for cycle-1282 D5
// (.evolve/runs/cycle-1279/audit-report.md, MEDIUM).
//
// failure_learning.go:442-448 states: "Only SELF-REPORTED structured defects are
// filed — the synthesized summary echo (ev.Defects == []string{summary}) is a
// restatement of the failure, not an actionable item, and filing it would be
// inbox noise." The guard implementing that claim is `structured != nil` ALONE.
// But `structured != nil` does not imply `len(structured.Defects) > 0`:
// phasecontract.ReadFailureBlock returns a block whenever Class != ""
// (sentinel.go:148), and ev.Defects is overwritten only under
// `if len(structured.Defects) > 0` (failure_learning.go:432). A
// classed-but-defectless block therefore leaves ev.Defects == []string{summary}
// and files it as a kind:"bug", priority:"H" inbox item — a comment asserting a
// filter the code does not implement.
//
// faillearn.structuredDefects (faillearn.go:143-148) is the EXISTING, named
// rule for exactly this degenerate case and is not reused. The builder must
// route the call site through that one rule rather than restating it here
// (feedback_never_duplicate_centralize_via_design_patterns) — exporting it from
// faillearn requires a matching entry in that package's apicover_named test.
//
// These drive writeDeterministicLearning, the seam the production failure path
// calls at failure_learning.go:366/372; TestC662_RetroCloseoutRecordsClosureInLedger
// already pins that RunCycle reaches it, so this file grades the RULE while the
// reachability lock stays where it is.

// remediationFixture builds the request writeDeterministicLearning needs plus a
// project root whose .evolve/inbox can be inspected afterwards.
func remediationFixture(t *testing.T) (*Orchestrator, failureLearningRequest, string) {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1279")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	o := &Orchestrator{now: func() time.Time { return time.Unix(1754000000, 0).UTC() }}
	fl := failureLearningRequest{
		CycleRequest: CycleRequest{ProjectRoot: root},
		Cycle:        1279,
		Failed:       PhaseAudit,
		CycleState:   &CycleState{WorkspacePath: ws},
	}
	return o, fl, root
}

// inboxFiles lists the .evolve/inbox entries the floor left behind.
func inboxFiles(t *testing.T, root string) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(root, ".evolve", "inbox"))
	if err != nil {
		return nil // no inbox at all is the "filed nothing" state
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

// TestWriteDeterministicLearning_ClassedButDefectlessBlockFilesNothing — D5.
// The degenerate case the comment claims is filtered: a failure block with a
// class and an EMPTY defects list. ev.Defects stays as the synthesized summary
// echo, and the summary echo must not become a priority-H inbox bug.
func TestWriteDeterministicLearning_ClassedButDefectlessBlockFilesNothing(t *testing.T) {
	o, fl, root := remediationFixture(t)

	o.writeDeterministicLearning(fl,
		"audit phase exited 1 after 3 attempts",
		&phasecontract.FailureBlock{Class: "deliverable-rejected"}, // classed, ZERO defects
	)

	if files := inboxFiles(t, root); len(files) != 0 {
		t.Errorf("the synthesized summary echo was filed as %v — failure_learning.go:442-448 asserts a filter the `structured != nil` guard does not implement; reuse faillearn's structuredDefects rule at the call site", files)
	}
	// The retrospective itself must still be written: the filter removes inbox
	// noise, never the lesson.
	if _, err := os.Stat(filepath.Join(fl.CycleState.WorkspacePath, "retrospective-report.md")); err != nil {
		t.Errorf("the retrospective must still be written when nothing is filed: %v", err)
	}
}

// TestWriteDeterministicLearning_EchoDefectListFilesNothing — the same rule via
// the other reachable shape: a block that self-reports exactly one defect which
// IS the summary. structuredDefects (faillearn.go:143-148) treats that as
// no-content; the inbox path must agree, or the two disagree about what a real
// defect is.
func TestWriteDeterministicLearning_EchoDefectListFilesNothing(t *testing.T) {
	o, fl, root := remediationFixture(t)
	summary := "audit phase exited 1 after 3 attempts"

	o.writeDeterministicLearning(fl, summary,
		&phasecontract.FailureBlock{Class: "deliverable-rejected", Defects: []string{summary}},
	)

	if files := inboxFiles(t, root); len(files) != 0 {
		t.Errorf("a defect list that merely echoes the summary was filed as %v — it is a restatement of the failure, not an actionable item", files)
	}
}

// TestWriteDeterministicLearning_StructuredDefectsAreFiled — the POSITIVE half.
// The filter must not swallow the case F1(ii) exists to fix: real self-reported
// defects still become addressable inbox items, one per defect, with non-empty
// provenance (inboxbatch.ConsoleRouted treats an empty injected_by as
// operator-authored).
func TestWriteDeterministicLearning_StructuredDefectsAreFiled(t *testing.T) {
	o, fl, root := remediationFixture(t)

	o.writeDeterministicLearning(fl,
		"audit phase exited 1 after 3 attempts",
		&phasecontract.FailureBlock{
			Class: "deliverable-rejected",
			Defects: []string{
				"reconcile truncate-writes the ledger from ancestor entries only",
				"closure evidence is validated for non-emptiness only",
			},
		},
	)

	files := inboxFiles(t, root)
	if len(files) != 2 {
		t.Fatalf("real self-reported defects must each reach the queue; inbox held %v (want 2 items)", files)
	}
}
