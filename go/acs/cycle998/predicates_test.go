//go:build acs

// Package cycle998 materialises the cycle-998 acceptance criteria for the two
// fleet-scoped carryover-consolidation tasks pinned to this lane:
//
//   - carryover-decisions-authoring   → author the judgment artifact AND apply it
//   - carryover-sweep-group-filer     → the surviving-items sweep-group re-filing
//
// What changed vs cycle-997. Cycle-997 built the machinery (`evolve carryover
// apply-decisions`, landed at go/cmd/evolve/cmd_carryover.go in this tree) and
// filed the six sweep-group inbox JSONs — but the decisions artifact it consumes
// never landed here, so nothing has run the apply and state.json:carryoverTodos
// is STILL 135 entries. The cycle-998 acceptance bar is therefore not "author a
// file" but "converge the live array": the decisions file must exist AND
// `--apply` must have run, so the drop/cluster ids are physically ABSENT from
// state.json. That applied effect is the crux predicate (003) — a re-author that
// skips `--apply` leaves every drop/cluster id resident and fails it.
//
// Predicate strategy — each predicate exercises a REAL emitted artifact or the
// APPLIED runtime state, never a source-grep of production code (the cycle-85
// degenerate-predicate ban):
//
//   - 001 parses the emitted decisions JSON and asserts it is internally valid
//     and does real pruning work (valid enum, non-empty reason, unique ids,
//     >= 60 drops, every cluster row names a cluster_group).
//   - 002 cross-references the decisions file against the LIVE state.json id set:
//     every surviving carryover id must be classified (survivors are `keep`).
//   - 003 is the cycle-998 crux: it reads the APPLIED state.json and asserts
//     every `drop`/`cluster` id is GONE and the live count fell well below the
//     135 baseline — proof the `--apply` step actually ran, not just authored.
//   - 004 cross-references the emitted sweep-group inbox JSONs against the
//     decisions file's `cluster` rows: exact 1:1 coverage, size 4–6, weight
//     0.7–0.8.
//
// Root resolution mirrors the cycle-997 predicates: artifacts and runtime state
// are read under acsassert.RepoRoot (the worktree, where Builder writes/applies
// per worktree isolation). The decisions file and applied state are Builder
// deliverables this cycle, so their absence / un-applied state is a FAILURE, not
// a skip.
package cycle998

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// decisionsRelPath is the emitted judgment artifact carryover-decisions-authoring
// must produce. Slug/date match scout-report's targetFiles and the CLI's default
// -decisions flag (cmd_carryover.go).
const decisionsRelPath = ".evolve/carryover-decisions-2026-07-21.json"

// carryoverBaseline is the live carryoverTodos count this cycle inherited
// (scout-report: still 135, un-shrunk). Post-apply the count MUST fall below it.
const carryoverBaseline = 135

// minDrops is the convergence floor: dropping < 60 of the 135 entries would leave
// the priority-inversion the inbox item names unaddressed (a keep-everything
// rubber-stamp).
const minDrops = 60

// carryoverDecision mirrors the on-disk schema the authoring task emits.
// cluster_group is required ONLY when decision=="cluster" (enforced below).
type carryoverDecision struct {
	ID           string `json:"id"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
	ClusterGroup string `json:"cluster_group"`
}

// decisionsFile matches cmd_carryover.go's carryoverDecisionsDoc: the rows live
// under a `decisions` array.
type decisionsFile struct {
	SourceCount int                 `json:"source_count"`
	Decisions   []carryoverDecision `json:"decisions"`
}

// loadDecisions reads + parses the emitted decisions artifact under the worktree
// root. It t.Fatalf's (RED) when the file is absent or unparseable — this
// artifact is a mandatory Builder deliverable this cycle (it never landed in
// cycle-997), so its absence is a failure, not a skip.
func loadDecisions(t *testing.T) decisionsFile {
	t.Helper()
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, decisionsRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("decisions artifact %s not readable (carryover-decisions-authoring must emit it): %v", decisionsRelPath, err)
	}
	var df decisionsFile
	if err := json.Unmarshal(raw, &df); err != nil {
		t.Fatalf("decisions artifact %s is not valid JSON: %v", decisionsRelPath, err)
	}
	if len(df.Decisions) == 0 {
		t.Fatalf("decisions artifact %s has an empty `decisions` array", decisionsRelPath)
	}
	return df
}

// liveCarryoverIDs reads the id set of state.json:carryoverTodos under the
// worktree root. Returns (ids, true) on success; (nil, false) when state.json is
// genuinely absent (cannot verify against a missing population).
func liveCarryoverIDs(t *testing.T) (map[string]bool, bool) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "state.json"))
	if err != nil {
		return nil, false
	}
	var state struct {
		CarryoverTodos []struct {
			ID string `json:"id"`
		} `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state.json is not valid JSON: %v", err)
	}
	ids := make(map[string]bool, len(state.CarryoverTodos))
	for _, e := range state.CarryoverTodos {
		if e.ID != "" {
			ids[e.ID] = true
		}
	}
	return ids, true
}

// TestC998_001_CarryoverDecisionsWellFormedAndPruning — AC (carryover-decisions-
// authoring), authoring half. The judgment artifact must be internally valid AND
// record real pruning work: every entry carries a valid decision enum
// (keep|drop|cluster) and a non-empty reason, ids are unique, each `cluster`
// entry names a non-empty cluster_group, and the file records >= 60 `drop`
// decisions. A "keep everything" no-op file fails the drop floor.
func TestC998_001_CarryoverDecisionsWellFormedAndPruning(t *testing.T) {
	df := loadDecisions(t)

	const validEnum = "keep|drop|cluster"
	seen := make(map[string]bool, len(df.Decisions))
	drops := 0
	for i, d := range df.Decisions {
		if d.ID == "" {
			t.Errorf("decision[%d] has empty id", i)
			continue
		}
		if seen[d.ID] {
			t.Errorf("decision id %q appears more than once (must be 1:1)", d.ID)
		}
		seen[d.ID] = true

		switch d.Decision {
		case "keep", "drop", "cluster":
			// valid
		default:
			t.Errorf("decision id %q has invalid decision %q (want one of %s)", d.ID, d.Decision, validEnum)
		}
		if strings.TrimSpace(d.Reason) == "" {
			t.Errorf("decision id %q has empty reason (every classification must justify itself)", d.ID)
		}
		if d.Decision == "drop" {
			drops++
		}
		if d.Decision == "cluster" && strings.TrimSpace(d.ClusterGroup) == "" {
			t.Errorf("decision id %q is `cluster` but names no cluster_group", d.ID)
		}
	}

	if drops < minDrops {
		t.Errorf("decisions file records only %d `drop` decisions; want >= %d to converge the %d-entry array", drops, minDrops, carryoverBaseline)
	}
}

// TestC998_002_CarryoverDecisionsCoverEveryLiveEntry — AC (carryover-decisions-
// authoring), the data-binding half. Every id currently in
// state.json:carryoverTodos MUST have exactly one decision row — no live entry
// left unclassified. This makes the artifact real: it cannot be satisfied by a
// plausible hand-written stub that omits the actual population. Robust to the
// apply shrinking the live array: survivors are `keep` entries, still covered.
func TestC998_002_CarryoverDecisionsCoverEveryLiveEntry(t *testing.T) {
	live, ok := liveCarryoverIDs(t)
	if !ok {
		t.Skip("state.json absent — cannot verify decision coverage against the live carryover population")
	}
	if len(live) == 0 {
		t.Skip("state.json carryoverTodos is empty — nothing to classify")
	}
	df := loadDecisions(t)
	classified := make(map[string]bool, len(df.Decisions))
	for _, d := range df.Decisions {
		classified[d.ID] = true
	}
	missing := 0
	for id := range live {
		if !classified[id] {
			missing++
			if missing <= 10 {
				t.Errorf("live carryoverTodos id %q has no decision row (unclassified)", id)
			}
		}
	}
	if missing > 10 {
		t.Errorf("... and %d more unclassified live ids (total %d of %d uncovered)", missing-10, missing, len(live))
	}
}

// TestC998_003_CarryoverDecisionsAppliedToState — the cycle-998 CRUX AC
// (carryover-decisions-authoring): authoring alone is NOT the deliverable; the
// `evolve carryover apply-decisions --apply` step must have RUN against this
// worktree's state.json. Proof of that is behavioural, read from the applied
// runtime state (not the authored file): every `drop` and every `cluster` id
// must be ABSENT from state.json:carryoverTodos, and the live count must have
// fallen well below the 135 baseline. A re-author that skips `--apply` leaves all
// drop/cluster ids resident and the count at 135 → this predicate stays RED.
func TestC998_003_CarryoverDecisionsAppliedToState(t *testing.T) {
	df := loadDecisions(t)
	live, ok := liveCarryoverIDs(t)
	if !ok {
		t.Fatalf("state.json absent — cannot verify the apply landed against the live carryover array")
	}

	removed := 0
	stillPresent := 0
	for _, d := range df.Decisions {
		if d.Decision != "drop" && d.Decision != "cluster" {
			continue
		}
		removed++
		if live[d.ID] {
			stillPresent++
			if stillPresent <= 10 {
				t.Errorf("id %q is classified %q but is STILL present in state.json:carryoverTodos — the `--apply` step did not run (or did not remove it)", d.ID, d.Decision)
			}
		}
	}
	if stillPresent > 10 {
		t.Errorf("... and %d more drop/cluster ids still resident (total %d of %d un-applied)", stillPresent-10, stillPresent, removed)
	}

	if len(live) >= carryoverBaseline {
		t.Errorf("state.json:carryoverTodos still holds %d entries (>= the %d baseline) — the consolidation apply did not shrink the array", len(live), carryoverBaseline)
	}
	if want := carryoverBaseline - minDrops; len(live) > want {
		t.Errorf("state.json:carryoverTodos holds %d entries; with >= %d drops applied it must fall to <= %d", len(live), minDrops, want)
	}
}

// sweepInboxFile mirrors the fields a sweep-group inbox filing carries. `items`
// is the list of clustered carryover ids the group amortises.
type sweepInboxFile struct {
	ID     string   `json:"id"`
	Weight float64  `json:"weight"`
	Kind   string   `json:"kind"`
	Items  []string `json:"items"`
}

// TestC998_004_SweepGroupsCoverEveryClusterExactlyOnce — AC (carryover-sweep-
// group-filer). Every `cluster` decision from the authoring artifact must land
// in EXACTLY ONE sweep-group inbox file (no orphan, no duplicate), each group
// must hold 4–6 items, each group must carry kind=sweep, and each group's weight
// must sit in the 0.7–0.8 band. Cross-references two emitted artifacts
// (decisions file ⋈ sweep files) — a genuine end-to-end binding, not a
// source-grep.
func TestC998_004_SweepGroupsCoverEveryClusterExactlyOnce(t *testing.T) {
	df := loadDecisions(t)
	wantCluster := make(map[string]bool)
	for _, d := range df.Decisions {
		if d.Decision == "cluster" {
			wantCluster[d.ID] = true
		}
	}
	if len(wantCluster) == 0 {
		t.Skip("decisions file classifies nothing as `cluster` — no sweep groups expected")
	}

	root := acsassert.RepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*carryover-sweep*.json"))
	if err != nil {
		t.Fatalf("glob sweep inbox files: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("no .evolve/inbox/*carryover-sweep*.json files present, but %d ids are classified `cluster` (carryover-sweep-group-filer must file them)", len(wantCluster))
	}

	covered := make(map[string]string) // clusterID -> sweep file that claims it
	for _, path := range matches {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read sweep file %s: %v", filepath.Base(path), err)
			continue
		}
		var sf sweepInboxFile
		if err := json.Unmarshal(raw, &sf); err != nil {
			t.Errorf("sweep file %s is not valid JSON: %v", filepath.Base(path), err)
			continue
		}
		if sf.Kind != "sweep" {
			t.Errorf("sweep file %s has kind %q; must be \"sweep\"", filepath.Base(path), sf.Kind)
		}
		if n := len(sf.Items); n < 4 || n > 6 {
			t.Errorf("sweep file %s has %d items; sweep groups must hold 4–6", filepath.Base(path), n)
		}
		if sf.Weight < 0.7 || sf.Weight > 0.8 {
			t.Errorf("sweep file %s has weight %.3f; must be in the 0.7–0.8 band", filepath.Base(path), sf.Weight)
		}
		for _, id := range sf.Items {
			if prev, dup := covered[id]; dup {
				t.Errorf("cluster id %q appears in two sweep files (%s and %s) — must be exactly once", id, filepath.Base(prev), filepath.Base(path))
			}
			covered[id] = path
		}
	}

	for id := range wantCluster {
		if _, ok := covered[id]; !ok {
			t.Errorf("cluster id %q is classified `cluster` but appears in no sweep-group inbox file (orphan)", id)
		}
	}
	for id := range covered {
		if !wantCluster[id] {
			t.Errorf("sweep files reference id %q which is NOT classified `cluster` in the decisions file", id)
		}
	}
}
