package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// writeLifecycleItem places an inbox todo in the lifecycle dir that makes
// inboxmover.ResolveDispatchState classify id as `state`. state=="pending" is
// the inbox root; state=="processing" lives under processing/cycle-N/.
func writeLifecycleItem(t *testing.T, evolveDir, state, id string, files ...string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox")
	switch state {
	case inboxmover.StatePending:
		// inbox root
	case inboxmover.StateProcessing:
		dir = filepath.Join(dir, "processing", "cycle-1181")
	default:
		dir = filepath.Join(dir, state)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{"id": id, "weight": 0.5, "files": files})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func decisionIDs(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	var doc struct {
		TopN []struct {
			ID string `json:"id"`
		} `json:"top_n"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decision is not parseable JSON: %v\n%s", err, data)
	}
	ids := map[string]bool{}
	for _, c := range doc.TopN {
		ids[c.ID] = true
	}
	return ids
}

func TestWidenNarrowDecision_DropsConsumedCommittedAtFleetWidth(t *testing.T) {
	dir := t.TempDir()
	writeLifecycleItem(t, dir, inboxmover.StateProcessed, "gone", "go/internal/x/a.go")
	writeLifecycleItem(t, dir, inboxmover.StatePending, "fresh", "go/internal/y/b.go")

	prior := []byte(`{"top_n":[{"id":"gone","files":["go/internal/x/a.go"]},{"id":"live","files":["go/internal/z/c.go"]}]}`)
	out := widenNarrowDecision(prior, dir, 2)
	ids := decisionIDs(t, out)

	if ids["gone"] {
		t.Errorf("consumed (processed) committed id `gone` survived the widen seam — it will be re-pinned into the next wave's lane-scope.json:\n%s", out)
	}
	if !ids["live"] {
		t.Errorf("still-live committed id `live` was dropped:\n%s", out)
	}
	if !ids["fresh"] {
		t.Errorf("the lane freed by pruning `gone` was not re-widened from the pending backlog (`fresh` absent):\n%s", out)
	}
}

func TestWidenNarrowDecision_ConsumedIDDroppedEvenWithNoBacklogReplacement(t *testing.T) {
	dir := t.TempDir()
	writeLifecycleItem(t, dir, inboxmover.StateRejected, "gone", "go/internal/x/a.go")

	prior := []byte(`{"top_n":[{"id":"gone","files":["go/internal/x/a.go"]},{"id":"live","files":["go/internal/z/c.go"]}]}`)
	out := widenNarrowDecision(prior, dir, 2)
	ids := decisionIDs(t, out)

	if ids["gone"] {
		t.Errorf("consumed (rejected) id `gone` survived because no backlog replacement existed — the plan must be honest even when it cannot be refilled:\n%s", out)
	}
	if !ids["live"] {
		t.Errorf("still-live committed id `live` was dropped:\n%s", out)
	}
}

func TestWidenNarrowDecision_PrunesEveryIDALaneCannotTake(t *testing.T) {
	cases := []struct {
		state    string
		wantKeep bool
	}{
		{inboxmover.StateProcessed, false},
		{inboxmover.StateRejected, false},
		{inboxmover.StateQuarantine, false},
		{inboxmover.StateConsumed, false},
		{inboxmover.StateProcessing, false},
		{inboxmover.StateRetry, false},
		{inboxmover.StatePending, true},
		{"no-evidence", true},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			dir := t.TempDir()
			if tc.state != "no-evidence" {
				writeLifecycleItem(t, dir, tc.state, "subject", "go/internal/x/a.go")
			}
			prior := []byte(`{"top_n":[{"id":"subject","files":["go/internal/x/a.go"]},{"id":"anchor","files":["go/internal/z/c.go"]}]}`)
			out := widenNarrowDecision(prior, dir, 2)
			ids := decisionIDs(t, out)

			if ids["subject"] != tc.wantKeep {
				t.Errorf("lifecycle state %q: `subject` present=%v, want %v:\n%s", tc.state, ids["subject"], tc.wantKeep, out)
			}
			if !ids["anchor"] {
				t.Errorf("lifecycle state %q: the dispatchable committed id `anchor` was dropped:\n%s", tc.state, out)
			}
		})
	}
}

func TestWaveNPlusOneExcludesConsumedScope(t *testing.T) {
	dir := t.TempDir()
	writeLifecycleItem(t, dir, inboxmover.StateProcessed, "consumed", "go/internal/x/a.go")
	writeLifecycleItem(t, dir, inboxmover.StatePending, "next-up", "go/internal/y/b.go")

	waveNDecision := []byte(`{"top_n":[{"id":"consumed","files":["go/internal/x/a.go"]},{"id":"keeper","files":["go/internal/z/c.go"]}]}`)
	waveNPlus1 := widenNarrowDecision(waveNDecision, dir, 2)

	specs, _, err := fleet.PlanFromTriage(waveNPlus1, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage over the wave N+1 decision: %v", err)
	}
	for lane, s := range specs {
		for _, id := range s.Scope {
			if id == "consumed" {
				t.Errorf("wave N+1 lane %d scope still carries the consumed id %q: %+v", lane, id, specs)
			}
		}
	}
	seen := map[string]bool{}
	for _, s := range specs {
		for _, id := range s.Scope {
			seen[id] = true
		}
	}
	if !seen["keeper"] {
		t.Errorf("wave N+1 dropped the still-live committed id `keeper`: %+v", specs)
	}
}
