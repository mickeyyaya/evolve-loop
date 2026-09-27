package main

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

func TestWidenNarrowDecision_UnparseableReturnsUnchanged(t *testing.T) {
	bad := []byte(`{not json`)
	if got := widenNarrowDecision(bad, t.TempDir(), 2); string(got) != string(bad) {
		t.Fatalf("unparseable decision must return unchanged; got %q", got)
	}
}

func TestMinWidthRepair_EmptyPlanAtFullCapacityRepairsNotSequential(t *testing.T) {
	launcher := &wsStarvationFakeLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 2},
		func() error { return nil }, planFn, launcher, nil, 0, &stderr, testRootSignals(t, &stderr))

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
