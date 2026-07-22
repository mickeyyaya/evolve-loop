package main

// cmd_loop_wave_starvation_test.go — cycle-557, task
// fix-wave-plan-source-starvation (scout-report.md ## Selected Tasks, Task 1).
//
// Two composed regressions this cycle removes, both of which force the wave
// planner off its isolated-lane path and onto the leak-prone in-supervisor
// sequential fallback (the standing rule `fleet_width_always_respected` calls
// this "the leak path"):
//
//  1. widenNarrowDecision returned a present-but-EMPTY prior triage decision
//     unchanged (the observed cycle-554 shape: top_n:[]). An empty top_n must
//     instead widen fully from the inbox backlog, exactly as an absent decision
//     would — otherwise the wave plans zero lanes from a non-empty backlog.
//  2. minWidthRepair's guard excluded the empty-plan-at-full-capacity shape
//     (fleetCfg.Count>1, waveCfg.Count>1, zero planned lanes) so it fell
//     through to sequential instead of repairing to one isolated lane.
import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type wsStarvationFakeLauncher struct{ calls [][]fleet.CycleSpec }

func (f *wsStarvationFakeLauncher) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	f.calls = append(f.calls, specs)
	out := make([]fleet.Result, len(specs))
	for i := range specs {
		out[i] = fleet.Result{Index: i, ExitCode: 0}
	}
	return out
}

// TestWidenNarrowDecision_EmptyTopNWidensFromInbox (regression #1). A prior
// decision with an EMPTY top_n plus a non-empty, file-disjoint inbox backlog
// must widen to fleet width — NOT return the empty decision unchanged. Before
// this cycle the `len(decision.TopN)==0` early-return returned `data` verbatim,
// starving the wave to zero lanes; this test fails against that code.
func TestWidenNarrowDecision_EmptyTopNWidensFromInbox(t *testing.T) {
	evolveDir := t.TempDir()
	inbox := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInbox := func(name, id string, weight float64, files ...string) {
		doc := map[string]any{"id": id, "weight": weight, "files": files}
		b, _ := json.Marshal(doc)
		if err := os.WriteFile(filepath.Join(inbox, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeInbox("a.json", "todo-a", 0.9, "pkg/a/a.go")
	writeInbox("b.json", "todo-b", 0.8, "pkg/b/b.go")

	out := widenNarrowDecision([]byte(`{"top_n":[]}`), evolveDir, 2)

	var got struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("widened output is not valid JSON: %v (%s)", err, out)
	}
	if len(got.TopN) != 2 {
		t.Fatalf("empty top_n + 2 disjoint inbox items must widen to 2 lanes; got %d (%s)", len(got.TopN), out)
	}
}

// TestWidenNarrowDecision_UnparseableReturnsUnchanged: only a genuinely
// unparseable decision returns the bytes verbatim — the widening is best-effort
// and must never corrupt a decision it cannot read.
func TestWidenNarrowDecision_UnparseableReturnsUnchanged(t *testing.T) {
	bad := []byte(`{not json`)
	if got := widenNarrowDecision(bad, t.TempDir(), 2); string(got) != string(bad) {
		t.Fatalf("unparseable decision must return unchanged; got %q", got)
	}
}

// TestMinWidthRepair_EmptyPlanAtFullCapacityRepairsNotSequential (regression
// #2). fleetCfg.Count>1 AND waveCfg.Count>1 (capacity held at full width) but
// dispatchIteration planned zero lanes: the repair must still fire (one
// isolated lane, handled=true), NOT fall through to sequential. The old guard
// `!(fleetCfg.Count>1 && waveCfg.Count<=1)` excluded this shape and WARNed
// "empty triage plan" with the launcher untouched — this test fails against it.
func TestMinWidthRepair_EmptyPlanAtFullCapacityRepairsNotSequential(t *testing.T) {
	launcher := &wsStarvationFakeLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 2},
		func() error { return nil }, planFn, launcher, nil, 0, &stderr)

	if !handled {
		t.Fatal("empty-plan-at-full-capacity (waveCfg.Count>1) must repair to one isolated lane, not fall through to sequential")
	}
	if len(launcher.calls) != 1 {
		t.Fatalf("repair must dispatch exactly one isolated lane; got %d launcher calls", len(launcher.calls))
	}
	if strings.Contains(stderr.String(), "empty triage plan") {
		t.Errorf("full-capacity empty plan must not be reported as an ineligible guard; got %q", stderr.String())
	}
}
