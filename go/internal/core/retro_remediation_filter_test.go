package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

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

func inboxFiles(t *testing.T, root string) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(root, ".evolve", "inbox"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return names
}

func TestWriteDeterministicLearning_ClassedButDefectlessBlockFilesNothing(t *testing.T) {
	t.Parallel()

	o, fl, root := remediationFixture(t)

	o.writeDeterministicLearning(fl,
		"audit phase exited 1 after 3 attempts",
		&phasecontract.FailureBlock{Class: "deliverable-rejected"},
	)

	if files := inboxFiles(t, root); len(files) != 0 {
		t.Errorf("the synthesized summary echo was filed as %v — failure_learning.go:442-448 asserts a filter the `structured != nil` guard does not implement; reuse faillearn's structuredDefects rule at the call site", files)
	}
	if _, err := os.Stat(filepath.Join(fl.CycleState.WorkspacePath, "retrospective-report.md")); err != nil {
		t.Errorf("the retrospective must still be written when nothing is filed: %v", err)
	}
}

func TestWriteDeterministicLearning_EchoDefectListFilesNothing(t *testing.T) {
	t.Parallel()

	o, fl, root := remediationFixture(t)
	summary := "audit phase exited 1 after 3 attempts"

	o.writeDeterministicLearning(fl, summary,
		&phasecontract.FailureBlock{Class: "deliverable-rejected", Defects: []string{summary}},
	)

	if files := inboxFiles(t, root); len(files) != 0 {
		t.Errorf("a defect list that merely echoes the summary was filed as %v — it is a restatement of the failure, not an actionable item", files)
	}
}

func TestWriteDeterministicLearning_StructuredDefectsAreFiled(t *testing.T) {
	t.Parallel()

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
