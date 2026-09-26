package triagecap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRecorder_ShippedCycleAppendsWindow(t *testing.T) {
	ws := t.TempDir()
	artifact := "## top_n\n- coverage-x: Push swarmrunner, swarmplan coverage ≥98%\n"
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := Recorder(repoRoot(t))
	var st core.State
	rec(&st, 290, ws)
	if len(st.TriageThroughput) != 1 {
		t.Fatalf("window = %+v, want 1 entry", st.TriageThroughput)
	}
	if e := st.TriageThroughput[0]; e.Cycle != 290 || e.Floors != 2 {
		t.Errorf("entry = %+v, want {Cycle:290 Floors:2}", e)
	}
}

func TestRecorder_MissingArtifactIsNoOp(t *testing.T) {
	rec := Recorder(repoRoot(t))
	st := core.State{TriageThroughput: []core.TriageThroughputEntry{{Cycle: 281, Floors: 5}}}
	rec(&st, 291, t.TempDir())
	if len(st.TriageThroughput) != 1 || st.TriageThroughput[0].Cycle != 281 {
		t.Errorf("window changed on missing artifact: %+v", st.TriageThroughput)
	}
}

func TestRecorder_ZeroFloorCycleIsNoOp(t *testing.T) {
	ws := t.TempDir()
	artifact := "## top_n\n- fix-bug: Fix the dispatch worktree bug\n"
	if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(artifact), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := Recorder(repoRoot(t))
	var st core.State
	rec(&st, 292, ws)
	if len(st.TriageThroughput) != 0 {
		t.Errorf("window = %+v, want empty (zero floors recorded)", st.TriageThroughput)
	}
}
