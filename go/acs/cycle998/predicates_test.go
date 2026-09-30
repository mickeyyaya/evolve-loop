//go:build acs

package cycle998

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const decisionsRelPath = ".evolve/carryover-decisions-2026-07-21.json"

const carryoverBaseline = 135

const minDrops = 60

type carryoverDecision struct {
	ID           string `json:"id"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
	ClusterGroup string `json:"cluster_group"`
}

type decisionsFile struct {
	SourceCount int                 `json:"source_count"`
	Decisions   []carryoverDecision `json:"decisions"`
}

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

type sweepInboxFile struct {
	ID     string   `json:"id"`
	Weight float64  `json:"weight"`
	Kind   string   `json:"kind"`
	Items  []string `json:"items"`
}

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

	covered := make(map[string]string)
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
