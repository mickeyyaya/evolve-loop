// cmd_loop_wave_menu_test.go — the seed and widen paths hand PlanFromTriage a
// MENU pool, not pre-flattened single reps (fleet-lane-batch-menu). Two
// defects pinned here, both proven live on batch-14 wave-1:
//
//  1. seedWavePlanFromInbox dropped candidate FILES from the synthesized
//     top_n cards, so fleet.Partition saw every card as an independent island
//     — it could never cluster same-file items into one lane, and (worse) it
//     could SPREAD two same-file items across two concurrent lanes.
//  2. Both paths carried exactly one id per lane, so a lane could never
//     amortize its worktree/build/audit across the cluster the batching layer
//     deliberately groups.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func writeInboxItemFiles(t *testing.T, inbox, name, id string, weight float64, files ...string) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"id": id, "weight": weight, "files": files})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestSeedWavePlanFromInbox_CardsCarryFilesAndClusterIntoLanes drives the
// synthesized decision through the REAL fleet.PlanFromTriage: two same-file
// clusters must become two lanes whose Scope carries the whole cluster, and
// the same-file pair must never split across lanes.
func TestSeedWavePlanFromInbox_CardsCarryFilesAndClusterIntoLanes(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItemFiles(t, inbox, "a1.json", "a1", 0.90, "go/internal/x/a.go")
	writeInboxItemFiles(t, inbox, "a2.json", "a2", 0.70, "go/internal/x/a.go")
	writeInboxItemFiles(t, inbox, "b1.json", "b1", 0.80, "go/internal/y/b.go")

	data, err := seedWavePlanFromInbox(dir, 2)
	if err != nil {
		t.Fatalf("seedWavePlanFromInbox: %v", err)
	}
	specs, _, err := fleet.PlanFromTriage(data, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage over the seed: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("lanes = %d, want 2: %+v", len(specs), specs)
	}
	byLane := map[string]int{}
	for lane, s := range specs {
		for _, id := range s.Scope {
			byLane[id] = lane
		}
	}
	if byLane["a1"] != byLane["a2"] {
		t.Errorf("a1 (lane %d) and a2 (lane %d) share go/internal/x/a.go but landed in DIFFERENT concurrent lanes — the seed dropped card files, so Partition could not see the collision", byLane["a1"], byLane["a2"])
	}
	if byLane["b1"] == byLane["a1"] {
		t.Errorf("b1 must own its own lane, got lane %d shared with a1", byLane["b1"])
	}
	// The menu itself: lane a's scope carries the whole cluster.
	laneA := specs[byLane["a1"]]
	if len(laneA.Scope) < 2 {
		t.Errorf("lane a Scope = %v, want the full same-file cluster (a1 + a2) as its menu", laneA.Scope)
	}
}

// TestWidenNarrowDecision_ExpandsCommittedLaneWithClusterMates pins the
// mid-batch path: a narrow prior decision (one committed id) widens to fleet
// width AND each lane deepens with its pending same-file mates from the inbox
// backlog, files preserved on every card.
func TestWidenNarrowDecision_ExpandsCommittedLaneWithClusterMates(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	writeInboxItemFiles(t, inbox, "mate.json", "mate", 0.70, "go/internal/x/a.go")
	writeInboxItemFiles(t, inbox, "other.json", "other", 0.80, "go/internal/y/b.go")

	prior := []byte(`{"top_n":[{"id":"committed","files":["go/internal/x/a.go"]}]}`)
	out := widenNarrowDecision(prior, dir, 2)

	specs, _, err := fleet.PlanFromTriage(out, nil, 2, nil)
	if err != nil {
		t.Fatalf("PlanFromTriage over the widened decision: %v", err)
	}
	byLane := map[string]int{}
	for lane, s := range specs {
		for _, id := range s.Scope {
			byLane[id] = lane
		}
	}
	if _, ok := byLane["committed"]; !ok {
		t.Fatalf("committed id dropped by widening: %+v", specs)
	}
	if _, ok := byLane["other"]; !ok {
		t.Fatalf("disjoint backlog item must widen the wave to a second lane: %+v", specs)
	}
	if lane, ok := byLane["mate"]; !ok {
		t.Errorf("pending same-file mate did not join the committed lane's menu: %+v", specs)
	} else if lane != byLane["committed"] {
		t.Errorf("mate (lane %d) must share committed's lane %d — they touch the same file", lane, byLane["committed"])
	}
}

// TestWidenNarrowDecision_CommittedFloorsShortCircuit (diff-review HIGH-1): a
// decision carrying committed_floors must pass through BYTE-IDENTICAL.
// fleet.TodosFromTriage's first switch case dispatches floors and ignores
// top_n entirely, so widening/deepening top_n cannot change the wave — but
// the re-marshal here rebuilds ONLY {"top_n":...}, so it would silently DROP
// the floors and flip the planner onto the top_n source. Mate expansion made
// that loss reachable in shapes the width-only guard used to pass through.
func TestWidenNarrowDecision_CommittedFloorsShortCircuit(t *testing.T) {
	dir := t.TempDir()
	inbox := filepath.Join(dir, "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	// A same-file mate AND a disjoint item: both expansion and widening would
	// fire if the floors guard is missing.
	writeInboxItemFiles(t, inbox, "mate.json", "mate", 0.70, "go/internal/x/a.go")
	writeInboxItemFiles(t, inbox, "other.json", "other", 0.80, "go/internal/y/b.go")

	prior := []byte(`{"committed_floors":["floor-a"],"top_n":[{"id":"t1","files":["go/internal/x/a.go"]}]}`)
	out := widenNarrowDecision(prior, dir, 2)
	if string(out) != string(prior) {
		t.Errorf("decision with committed_floors was rewritten:\n got: %s\nwant: %s\n— the re-marshal drops committed_floors and floor-a would never dispatch", out, prior)
	}
}
