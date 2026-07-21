package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
)

// TestCarryoverSubcommandRegistered — the `carryover` command is wired into the
// dispatcher table (an unregistered command is dead code the CLI can never
// route to).
func TestCarryoverSubcommandRegistered(t *testing.T) {
	found := false
	for _, c := range commands {
		if c.Name == "carryover" {
			found = true
			if c.Run == nil {
				t.Fatalf("carryover command registered with a nil Run handler")
			}
		}
	}
	if !found {
		t.Fatalf("carryover subcommand is not registered in the commands table")
	}
}

// writeFixtureState writes a state.json with the given carryover ids and returns
// its path.
func writeFixtureState(t *testing.T, ids ...string) string {
	t.Helper()
	dir := t.TempDir()
	todos := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		todos = append(todos, map[string]any{"id": id})
	}
	state := map[string]any{"carryoverTodos": todos, "someOtherKey": "preserved"}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture state: %v", err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fixture state: %v", err)
	}
	return path
}

func readCarryoverIDs(t *testing.T, statePath string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var state struct {
		CarryoverTodos []struct {
			ID string `json:"id"`
		} `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state.json corrupted / not valid JSON: %v", err)
	}
	ids := make(map[string]bool)
	for _, e := range state.CarryoverTodos {
		ids[e.ID] = true
	}
	return ids
}

// TestCarryoverApplyDecisions_DropsEntriesToCeiling — applying a fixture
// decisions set removes the drop + cluster ids and lands the live count under
// the ceiling (the actual convergence behaviour the inbox item asks for).
func TestCarryoverApplyDecisions_DropsEntriesToCeiling(t *testing.T) {
	// 30 entries: keep 3, drop 25, cluster 2 → survivors = 3, well under 25.
	ids := make([]string, 0, 30)
	doc := carryoverDecisionsDoc{SourceCount: 30}
	for i := 0; i < 30; i++ {
		id := "todo-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		ids = append(ids, id)
		var decision, group string
		switch {
		case i < 3:
			decision = "keep"
		case i < 28:
			decision = "drop"
		default:
			decision, group = "cluster", "sweep-x"
		}
		doc.Decisions = append(doc.Decisions, carryoverDecisionRow{
			ID: id, Decision: decision, Reason: "fixture reason", ClusterGroup: group,
		})
	}
	statePath := writeFixtureState(t, ids...)

	res, err := applyCarryoverDecisions(statePath, doc)
	if err != nil {
		t.Fatalf("applyCarryoverDecisions: %v", err)
	}
	if res.Before != 30 {
		t.Errorf("Before = %d, want 30", res.Before)
	}
	if res.Dropped != 25 {
		t.Errorf("Dropped = %d, want 25", res.Dropped)
	}
	if res.Clustered != 2 {
		t.Errorf("Clustered = %d, want 2", res.Clustered)
	}
	if res.After != 3 {
		t.Errorf("After = %d, want 3", res.After)
	}
	if res.After > carryoverApplyCeiling {
		t.Errorf("After = %d exceeds ceiling %d", res.After, carryoverApplyCeiling)
	}
	surviving := readCarryoverIDs(t, statePath)
	if len(surviving) != 3 {
		t.Errorf("surviving id count = %d, want 3", len(surviving))
	}
	// Every dropped/clustered id must be gone; every keep id must remain.
	for _, d := range doc.Decisions {
		switch d.Decision {
		case "drop", "cluster":
			if surviving[d.ID] {
				t.Errorf("id %q was %s but survived", d.ID, d.Decision)
			}
		case "keep":
			if !surviving[d.ID] {
				t.Errorf("id %q was keep but was removed", d.ID)
			}
		}
	}
}

// TestCarryoverApplyDecisions_RejectsMissingReason — NEGATIVE. A decision row
// with an empty reason is rejected and state.json is left UNMUTATED (the
// anti-hand-edit / anti-unjustified-drop guard). Validation must run before any
// lock or write.
func TestCarryoverApplyDecisions_RejectsMissingReason(t *testing.T) {
	statePath := writeFixtureState(t, "todo-keep-me", "todo-drop-me")
	before := readCarryoverIDs(t, statePath)
	beforeRaw, _ := os.ReadFile(statePath)

	doc := carryoverDecisionsDoc{
		SourceCount: 2,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-keep-me", Decision: "keep", Reason: "still live"},
			{ID: "todo-drop-me", Decision: "drop", Reason: "  "}, // whitespace-only → empty
		},
	}
	if err := validateCarryoverDecisions(doc); err == nil {
		t.Fatalf("validateCarryoverDecisions accepted a row with an empty reason")
	}

	// The full command path must also refuse without mutating state.json.
	code := runCarryoverApplyDecisions([]string{"--apply", "--state", statePath, "--decisions", writeDecisionsFile(t, doc)}, os.Stderr, os.Stderr)
	if code == 0 {
		t.Fatalf("runCarryoverApplyDecisions returned 0 for an empty-reason decisions file")
	}
	afterRaw, _ := os.ReadFile(statePath)
	if string(beforeRaw) != string(afterRaw) {
		t.Fatalf("state.json was mutated despite a rejected decisions file")
	}
	after := readCarryoverIDs(t, statePath)
	if len(after) != len(before) {
		t.Fatalf("carryover count changed on a rejected apply: %d → %d", len(before), len(after))
	}
}

// writeDecisionsFile writes doc to a temp decisions JSON and returns its path.
func writeDecisionsFile(t *testing.T, doc carryoverDecisionsDoc) string {
	t.Helper()
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal decisions: %v", err)
	}
	path := filepath.Join(t.TempDir(), "decisions.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write decisions: %v", err)
	}
	return path
}

// TestCarryoverApplyDecisions_UsesLockedRMW — the apply goes through the
// sanctioned flock.WithPathLock RMW path (no ad-hoc unlocked write). Proven two
// ways: (1) the sidecar lock file `<statePath>.lock` is materialised by the
// PathLock code path, and (2) concurrent applies serialise without corrupting
// the array (run under -race). An unlocked implementation fails both: no sidecar
// and/or a torn/duplicated final state.
func TestCarryoverApplyDecisions_UsesLockedRMW(t *testing.T) {
	statePath := writeFixtureState(t, "todo-a", "todo-b", "todo-keep")
	doc := carryoverDecisionsDoc{
		SourceCount: 3,
		Decisions: []carryoverDecisionRow{
			{ID: "todo-a", Decision: "drop", Reason: "stale"},
			{ID: "todo-b", Decision: "drop", Reason: "stale"},
			{ID: "todo-keep", Decision: "keep", Reason: "live"},
		},
	}

	if _, err := applyCarryoverDecisions(statePath, doc); err != nil {
		t.Fatalf("applyCarryoverDecisions: %v", err)
	}
	if _, err := os.Stat(statePath + ".lock"); err != nil {
		t.Fatalf("sidecar lock %s.lock was not created — apply did not go through flock.WithPathLock: %v", statePath, err)
	}

	// Concurrent applies (idempotent) must never corrupt the file. Under -race
	// + the flock serialization this converges to a single valid state.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = applyCarryoverDecisions(statePath, doc)
		}()
	}
	wg.Wait()

	surviving := readCarryoverIDs(t, statePath) // fatals if JSON is torn
	if !surviving["todo-keep"] || surviving["todo-a"] || surviving["todo-b"] {
		t.Fatalf("post-concurrency state wrong: %v", surviving)
	}
}

// TestCarryoverApplyDecisions_PreservesStateSymlink is the cycle-999
// regression pin: applying decisions through a WORKTREE-style symlinked
// state.json must write THROUGH to the canonical target and leave the link
// intact — the pre-fix hand-rolled temp+rename replaced the link with a
// detached regular file, stranding the 135->14 convergence in a dead copy.
func TestCarryoverApplyDecisions_PreservesStateSymlink(t *testing.T) {
	canonicalDir := t.TempDir()
	worktreeDir := t.TempDir()
	canonical := filepath.Join(canonicalDir, "state.json")
	seed := map[string]any{
		"stateRevision": float64(3),
		"carryoverTodos": []any{
			map[string]any{"id": "keep-me"},
			map[string]any{"id": "drop-me"},
		},
	}
	raw, _ := json.Marshal(seed)
	if err := os.WriteFile(canonical, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(worktreeDir, "state.json")
	if err := os.Symlink(canonical, link); err != nil {
		t.Fatal(err)
	}

	doc := carryoverDecisionsDoc{Decisions: []carryoverDecisionRow{
		{ID: "drop-me", Decision: "drop", Reason: "superseded by regression pin"},
	}}
	res, err := applyCarryoverDecisions(link, doc)
	if err != nil {
		t.Fatalf("apply through symlink: %v", err)
	}
	if res.Before != 2 || res.After != 1 || res.Dropped != 1 {
		t.Fatalf("res=%+v, want Before=2 After=1 Dropped=1", res)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("state.json symlink SEVERED by apply (the cycle-999 defect)")
	}
	got, err := statemap.ReadStateMap(canonical)
	if err != nil {
		t.Fatal(err)
	}
	arr, _ := got["carryoverTodos"].([]any)
	if len(arr) != 1 {
		t.Fatalf("canonical not updated through link: %v", got["carryoverTodos"])
	}
}
