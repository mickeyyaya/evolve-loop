package triagecap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cand(id string, weight float64, files ...string) FleetCandidate {
	return FleetCandidate{ID: id, Weight: weight, Files: files}
}

func TestExpandWithClusterMates_AddsSameFileMatesUpToCap(t *testing.T) {
	sel := []FleetCandidate{cand("rep", 0.9, "go/internal/x/a.go")}
	backlog := []FleetCandidate{
		cand("rep", 0.9, "go/internal/x/a.go"),
		cand("m1", 0.8, "go/internal/x/a.go"),
		cand("m2", 0.7, "go/internal/x/a.go"),
		cand("m3", 0.6, "go/internal/x/a.go"),
		cand("m4", 0.5, "go/internal/x/a.go"),
	}
	menus := ExpandWithClusterMates(sel, backlog, 4)
	if len(menus) != 1 {
		t.Fatalf("menus = %d, want 1 (expansion deepens lanes, never widens)", len(menus))
	}
	if got := menuIDs(menus[0]); got != "rep,m1,m2,m3" {
		t.Errorf("menu = %s, want rep,m1,m2,m3 — highest-weight mates first, m4 dropped by the perLane cap", got)
	}
}

func TestExpandWithClusterMates_BridgeNeverJoins(t *testing.T) {
	sel := []FleetCandidate{
		cand("lane-a", 0.9, "go/internal/x/a.go"),
		cand("lane-b", 0.8, "go/internal/y/b.go"),
	}
	backlog := append([]FleetCandidate{}, sel...)
	backlog = append(backlog, cand("bridge", 0.7, "go/internal/x/a.go", "go/internal/y/b.go"))
	menus := ExpandWithClusterMates(sel, backlog, 4)
	for _, m := range menus {
		if strings.Contains(menuIDs(m), "bridge") {
			t.Fatalf("bridge candidate joined a lane: %v", menus)
		}
	}
}

func TestExpandWithClusterMates_IndependentWorkNeverPads(t *testing.T) {
	sel := []FleetCandidate{cand("rep", 0.9, "go/internal/x/a.go")}
	backlog := []FleetCandidate{
		cand("rep", 0.9, "go/internal/x/a.go"),
		cand("unrelated", 0.8, "docs/other.md"),
	}
	menus := ExpandWithClusterMates(sel, backlog, 4)
	if got := menuIDs(menus[0]); got != "rep" {
		t.Errorf("menu = %s, want rep alone — unrelated backlog must not pad a lane", got)
	}
}

func TestExpandWithClusterMates_CrossLaneFilesStayDisjoint(t *testing.T) {
	sel := []FleetCandidate{
		cand("lane-a", 0.9, "go/internal/x/a.go"),
		cand("lane-b", 0.8, "go/internal/y/b.go"),
	}
	backlog := []FleetCandidate{
		cand("m1", 0.7, "go/internal/x/a.go", "go/internal/x/extra.go"),
		// m2 touches m1's extra file (claimed once m1 joins lane-a) and lane-b's file.
		cand("m2", 0.6, "go/internal/x/extra.go", "go/internal/y/b.go"),
	}
	menus := ExpandWithClusterMates(sel, backlog, 4)
	fileOwner := map[string]int{}
	idSeen := map[string]bool{}
	for lane, m := range menus {
		for _, c := range m {
			if idSeen[c.ID] {
				t.Fatalf("id %s appears in two lanes: %v", c.ID, menus)
			}
			idSeen[c.ID] = true
			for _, f := range c.Files {
				f = filepath.Clean(f)
				if prev, ok := fileOwner[f]; ok && prev != lane {
					t.Fatalf("file %s owned by lanes %d and %d — cross-lane collision: %v", f, prev, lane, menus)
				}
				fileOwner[f] = lane
			}
		}
	}
	if idSeen["m2"] {
		t.Fatalf("m2 joined a lane despite bridging lane-a's grown file set and lane-b: %v", menus)
	}
}

func TestSelectWaveSeedMenus_EndToEnd(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "a1", 0.9, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "a2", 0.7, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "b1", 0.8, "go/internal/y/b.go")
	writeInboxTodo(t, evolveDir, "b2", 0.6, "go/internal/y/b.go")

	menus := SelectWaveSeedMenus(evolveDir, nil, 2, 4, nil)
	if len(menus) != 2 {
		t.Fatalf("menus = %d, want 2 lanes", len(menus))
	}
	got := menuIDs(menus[0]) + " | " + menuIDs(menus[1])
	if got != "a1,a2 | b1,b2" {
		t.Errorf("menus = %s, want a1,a2 | b1,b2 (each lane consumes its whole cluster, weight-ordered)", got)
	}
}

func TestSelectWaveSeedMenus_IsDeterministic(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "a1", 0.9, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "a2", 0.9, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "b1", 0.9, "go/internal/y/b.go")
	first := renderMenus(SelectWaveSeedMenus(evolveDir, nil, 2, 4, nil))
	for i := 0; i < 10; i++ {
		if got := renderMenus(SelectWaveSeedMenus(evolveDir, nil, 2, 4, nil)); got != first {
			t.Fatalf("run %d diverged: %s vs %s", i, got, first)
		}
	}
}

func menuIDs(m []FleetCandidate) string {
	ids := make([]string, len(m))
	for i, c := range m {
		ids[i] = c.ID
	}
	return strings.Join(ids, ",")
}

func renderMenus(menus [][]FleetCandidate) string {
	var parts []string
	for _, m := range menus {
		parts = append(parts, menuIDs(m))
	}
	return strings.Join(parts, " | ")
}

func writeInboxTodo(t *testing.T, evolveDir, id string, weight float64, files ...string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{"id": id, "weight": weight, "files": files})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExpandWithClusterMates_OverlappingSelectionFirstLaneKeepsFile(t *testing.T) {
	sel := []FleetCandidate{
		cand("first", 0.9, "go/internal/x/a.go"),
		cand("second", 0.8, "go/internal/x/a.go"), // a committed prefix may overlap itself
	}
	backlog := []FleetCandidate{cand("mate", 0.7, "go/internal/x/a.go")}
	menus := ExpandWithClusterMates(sel, backlog, 4)
	if got := menuIDs(menus[0]); got != "first,mate" {
		t.Errorf("menus[0] = %s, want first,mate — the FIRST lane owns the shared file, so the mate belongs to it", got)
	}
	if got := menuIDs(menus[1]); got != "second" {
		t.Errorf("menus[1] = %s, want second alone — a later overlapping rep must not accrete mates on a stolen claim", got)
	}
}

func TestSelectWaveSeedMenus_PreservesCommittedPrefix(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "a1", 0.9, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "a2", 0.7, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "b1", 0.8, "go/internal/y/b.go")
	writeInboxTodo(t, evolveDir, "c1", 0.1, "go/internal/z/c.go") // committed, lowest weight

	committed := []FleetCandidate{cand("c1", 0.1, "go/internal/z/c.go")}
	menus := SelectWaveSeedMenus(evolveDir, committed, 2, 4, nil)

	if len(menus) != 2 {
		t.Fatalf("menus = %d lanes, want 2 (committed prefix + one widened lane): %v", len(menus), renderMenus(menus))
	}
	if got := menus[0][0].ID; got != "c1" {
		t.Errorf("lane 0 rep = %q, want the committed id %q — committed intent must never be reordered behind a higher-weight backlog item; menus=%s", got, "c1", renderMenus(menus))
	}
	if got := menuIDs(menus[1]); got != "a1,a2" {
		t.Errorf("lane 1 = %q, want a1,a2 (widened from backlog, deepened with its same-file mate); menus=%s", got, renderMenus(menus))
	}
}

func TestSelectWaveSeedMenus_CommittedNotInBacklogStillSeeded(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "a1", 0.9, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "b1", 0.8, "go/internal/y/b.go")

	committed := []FleetCandidate{cand("gone", 0.5, "go/internal/z/gone.go")} // absent from the inbox
	menus := SelectWaveSeedMenus(evolveDir, committed, 2, 4, nil)

	if len(menus) == 0 || menus[0][0].ID != "gone" {
		t.Fatalf("committed id absent from the inbox must still seed lane 0; menus=%s", renderMenus(menus))
	}
}

func TestSelectWaveSeedMenus_CommittedOverlapNeverDropped(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "a1", 0.9, "go/internal/x/a.go")

	committed := []FleetCandidate{
		cand("c1", 0.4, "go/internal/z/c.go"),
		cand("c2", 0.3, "go/internal/z/c.go"), // overlaps c1 on purpose
	}
	menus := SelectWaveSeedMenus(evolveDir, committed, 2, 4, nil)

	seen := map[string]bool{}
	for _, m := range menus {
		for _, c := range m {
			seen[c.ID] = true
		}
	}
	for _, id := range []string{"c1", "c2"} {
		if !seen[id] {
			t.Errorf("committed id %q was dropped; menus=%s", id, renderMenus(menus))
		}
	}
}

func TestSelectWaveSeedMenus_NilCommittedMatchesLegacySelection(t *testing.T) {
	evolveDir := t.TempDir()
	writeInboxTodo(t, evolveDir, "a1", 0.9, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "a2", 0.7, "go/internal/x/a.go")
	writeInboxTodo(t, evolveDir, "b1", 0.8, "go/internal/y/b.go")
	writeInboxTodo(t, evolveDir, "b2", 0.6, "go/internal/y/b.go")

	backlog := ReadInboxBacklog(evolveDir, nil)
	want := renderMenus(ExpandWithClusterMates(SelectFleetWidthTopN(backlog, 2), backlog, 4))
	if got := renderMenus(SelectWaveSeedMenus(evolveDir, nil, 2, 4, nil)); got != want {
		t.Errorf("nil committed = %q, want legacy %q — an empty prefix must not change the selection", got, want)
	}
	if want != "a1,a2 | b1,b2" {
		t.Errorf("legacy baseline drifted: %q", want)
	}
}
