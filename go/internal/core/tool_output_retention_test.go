package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func seedToolOutput(t *testing.T, ws string) []string {
	t.Helper()
	var paths []string
	for _, name := range append(gcpolicy.ToolOutputFiles(), "build-report.md", "tdd-stdout.log") {
		p := filepath.Join(ws, name)
		if err := os.WriteFile(p, []byte("=== RUN   TestX\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

func sealCycle(t *testing.T, verdict string, shipped bool) (CycleResult, string) {
	t.Helper()
	ws := t.TempDir()
	seedToolOutput(t, ws)
	o := &Orchestrator{storage: &fakeUpdaterStorage{}, gitHEAD: func() (string, error) { return "same-head", nil }}
	cr := &cycleRun{
		ctx:    context.Background(),
		o:      o,
		cycle:  7,
		req:    CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "tool-output-retention"},
		cs:     CycleState{WorkspacePath: ws, Shipped: shipped},
		result: CycleResult{Cycle: 7, FinalVerdict: verdict, PhasesRun: []Phase{PhaseBuild, PhaseAudit, PhaseShip}},
	}
	if err := cr.completeCycle(); err != nil {
		t.Fatalf("completeCycle: %v", err)
	}
	return cr.result, ws
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestCompleteCycle_APassSealDeletesTheRawToolOutputAndKeepsTheRest(t *testing.T) {
	res, ws := sealCycle(t, VerdictPASS, true)
	if res.FinalVerdict != VerdictPASS {
		t.Fatalf("precondition: the cycle must seal PASS, got %q", res.FinalVerdict)
	}

	for _, name := range gcpolicy.ToolOutputFiles() {
		if exists(filepath.Join(ws, name)) {
			t.Errorf("%s survived a PASS seal; only a failed cycle keeps raw tool output", name)
		}
	}
	for _, name := range []string{"build-report.md", "tdd-stdout.log"} {
		if !exists(filepath.Join(ws, name)) {
			t.Errorf("%s is not raw tool output, but the PASS seal deleted it", name)
		}
	}
}

func TestCompleteCycle_ASealThatIsNotPassKeepsTheRawToolOutput(t *testing.T) {
	for _, verdict := range []string{VerdictFAIL, VerdictWARN} {
		res, ws := sealCycle(t, verdict, false)
		if res.FinalVerdict == VerdictPASS {
			t.Fatalf("precondition: a %s cycle must not seal PASS", verdict)
		}

		for _, name := range gcpolicy.ToolOutputFiles() {
			if !exists(filepath.Join(ws, name)) {
				t.Errorf("%s was deleted at a %s seal; only a PASS seal deletes raw tool output", name, res.FinalVerdict)
			}
		}
	}
}

func TestPruneToolOutputOnPass_AMissingFileIsNotAnError(t *testing.T) {
	if err := pruneToolOutputOnPass(t.TempDir(), VerdictPASS); err != nil {
		t.Errorf("a PASS run dir with no raw tool output: err = %v, want nil", err)
	}
}

func TestPruneToolOutputOnPass_ARemoveFailureIsReturnedWithThePath(t *testing.T) {
	ws := t.TempDir()
	busy := filepath.Join(ws, gcpolicy.ToolOutputFiles()[0])
	if err := os.MkdirAll(filepath.Join(busy, "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := pruneToolOutputOnPass(ws, VerdictPASS)

	if err == nil || !strings.Contains(err.Error(), busy) {
		t.Errorf("err = %v, want an error that names %s", err, busy)
	}
}
