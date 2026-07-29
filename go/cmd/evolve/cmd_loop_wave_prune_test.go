// cmd_loop_wave_prune_test.go — cycle-1182 RED contract for
// wave-planner-pass-scope-prune.
//
// The defect: widenNarrowDecision is the PRIMARY per-wave planning path (it runs
// whenever a prior cycle's triage-decision.json exists — the common case, not
// just the first wave). It builds `committed` straight from decision.top_n and
// then either short-circuits (`len(committed) >= count`) or hands the list to
// WidenTopNToFleetWidth, which copies the committed prefix through VERBATIM.
// Neither branch consults the inbox lifecycle, so an id a previous cycle already
// CONSUMED survives in the prior decision file and gets re-pinned into the next
// wave's plan / lane-scope.json (cycle-1116 re-pinned tdd-topn-binding-gate after
// cycle-1113 consumed it). The sibling fresh-seed path
// (triagecap.SelectWaveSeedMenus) already prunes; only this seam does not.
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - Call the exported triagecap.PruneConsumed on `committed` immediately after
//     it is built from decision.TopN and BEFORE the `len(committed) >= count`
//     early-return, so a fleet-width-but-stale list is still cleaned and then
//     re-widened from the backlog.
//   - A prune that drops ids must never fall back to returning the ORIGINAL
//     bytes: the stale id would ride through the `len(topN) <= len(committed)`
//     guard untouched.
//   - The committed_floors byte-identical passthrough is unchanged (pinned by
//     the existing TestWidenNarrowDecision_CommittedFloorsShortCircuit).
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

// decisionIDs parses a decision blob's top_n ids.
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

// TestWidenNarrowDecision_DropsConsumedCommittedAtFleetWidth is the crux: the
// prior decision is ALREADY fleet-width (2 committed, count=2), so today's
// `len(committed) >= count` short-circuit returns it verbatim and the consumed
// id `gone` is re-pinned into the next wave. After the fix the consumed id must
// be absent and the freed lane re-widened from the pending backlog.
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

// TestWidenNarrowDecision_ConsumedIDDroppedEvenWithNoBacklogReplacement is the
// adversarial edge on the same criterion: with an EMPTY backlog there is nothing
// to widen with, so a naive fix falls through to the
// `len(topN) <= len(committed)` guard and returns the ORIGINAL bytes — carrying
// the consumed id forward exactly as before. Correctness (an honest plan) must
// win over the "nothing to add, leave as-is" optimization.
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

// TestWidenNarrowDecision_PrunesTerminalStatesOnly is the fail-open negative
// axis: only processed/rejected/quarantine may be pruned. pending, processing,
// retry and an id with NO lifecycle evidence at all must be retained — dropping
// unresolvable ids would starve every wave of non-inbox-backed cards.
func TestWidenNarrowDecision_PrunesTerminalStatesOnly(t *testing.T) {
	cases := []struct {
		state    string
		wantKeep bool
	}{
		{inboxmover.StateProcessed, false},
		{inboxmover.StateRejected, false},
		{inboxmover.StateQuarantine, false},
		{inboxmover.StatePending, true},
		{inboxmover.StateProcessing, true},
		{inboxmover.StateRetry, true},
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
				t.Errorf("lifecycle state %q: unrelated committed id `anchor` was dropped — the prune must only touch consumed ids:\n%s", tc.state, out)
			}
		})
	}
}

// TestWaveNPlusOneExcludesConsumedScope is the two-wave regression the inbox item
// describes, driven end-to-end through the REAL planner: wave N committed
// {consumed, keeper}; the cycle then consumed `consumed` (inbox → processed/).
// Wave N+1 re-plans from wave N's decision bytes — no lane may carry the consumed
// id in its scope.
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
