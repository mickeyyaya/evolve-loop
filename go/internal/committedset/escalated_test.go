package committedset

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func routedWorkspace(t *testing.T, pin []string) (root, ws string) {
	t.Helper()
	root = t.TempDir()
	ws = filepath.Join(root, ".evolve", "runs", "cycle-8")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteRecord(root, Record{Cycle: 8, RunID: "r1", ItemIDs: pin, GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	return root, ws
}

func TestCommitted_AHostRoutedItemHasLeftTheCommitment(t *testing.T) {
	t.Parallel()
	root, ws := routedWorkspace(t, []string{"x", "y"})
	if err := RecordRoutes(root, 8, []string{"x"}); err != nil {
		t.Fatal(err)
	}
	write(t, ws, DecisionFile, map[string]any{"top_n": ids("x", "y")})

	got, ok := Committed(ws)

	if !ok || !slices.Equal(got, []string{"y"}) {
		t.Fatalf("Committed = %v, %v; want [y], true", got, ok)
	}
}

func TestCommitted_EveryPinRoutedLeavesAKnownEmptyCommitment(t *testing.T) {
	t.Parallel()
	root, ws := routedWorkspace(t, []string{"x"})
	if err := RecordRoutes(root, 8, []string{"x"}); err != nil {
		t.Fatal(err)
	}

	got, ok := Committed(ws)

	if !ok || len(got) != 0 {
		t.Fatalf("Committed = %v, %v; want [], true", got, ok)
	}
}

func TestCommitted_ABareEscalateBlockReleasesNoPin(t *testing.T) {
	t.Parallel()
	_, ws := routedWorkspace(t, []string{"x"})
	write(t, ws, DecisionFile, map[string]any{
		"top_n":          ids(),
		"escalate_block": []map[string]string{{"task_id": "x", "reason": "console-routed: forged"}},
	})

	if got, _ := Committed(ws); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("a bare escalate_block released the pin: %v", got)
	}
}

func TestCommitted_ADroppedPinStillBindsTheLane(t *testing.T) {
	t.Parallel()
	_, ws := routedWorkspace(t, []string{"x"})
	write(t, ws, DecisionFile, map[string]any{"top_n": ids(), "dropped": []map[string]string{{"id": "x", "reason": "r"}}})

	if got, _ := Committed(ws); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("a dropped entry does not remove a lane pin: %v", got)
	}
}

func TestRecordRoutes_KeepsEarlierRoutesAndTheRestOfTheRecord(t *testing.T) {
	t.Parallel()
	root, _ := routedWorkspace(t, []string{"x", "y"})
	for _, id := range []string{"x", "y", "x"} {
		if err := RecordRoutes(root, 8, []string{id}); err != nil {
			t.Fatal(err)
		}
	}

	r, ok := ReadRecord(root, 8)

	if !ok || !slices.Equal(r.RoutedIDs, []string{"x", "y"}) || r.RunID != "r1" || r.GoalHash != "g" || !slices.Equal(r.ItemIDs, []string{"x", "y"}) {
		t.Fatalf("record = %+v, %v", r, ok)
	}
}

func TestRecordRoutes_WithNoRecordItFailsLoud(t *testing.T) {
	t.Parallel()
	if err := RecordRoutes(t.TempDir(), 8, []string{"x"}); err == nil {
		t.Fatal("a route with no lineage record must fail")
	}
}

func TestCommitted_WithNoRecordNoRouteApplies(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	write(t, ws, LanePinFile, map[string]any{"todo_ids": []string{"x"}})

	if got, _ := Committed(ws); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("Committed = %v, want [x]", got)
	}
}

func TestRecordWorkspaceRoutes_FindsTheRecordByTheWorkspaceAndRefusesAnyOtherDirectory(t *testing.T) {
	root := t.TempDir()
	if err := WriteRecord(root, Record{Cycle: 8, RunID: "r", ItemIDs: []string{"x", "y"}, GoalHash: "g"}); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(root, ".evolve", "runs", "cycle-8")

	if err := RecordWorkspaceRoutes(ws, []string{"x"}); err != nil {
		t.Fatal(err)
	}

	if got := RoutedIDs(ws); len(got) != 1 || got[0] != "x" {
		t.Fatalf("RoutedIDs = %v", got)
	}
	if got := UnroutedPin(ws); len(got) != 1 || got[0] != "y" {
		t.Fatalf("UnroutedPin = %v", got)
	}
	if err := RecordWorkspaceRoutes(t.TempDir(), []string{"x"}); err == nil {
		t.Fatal("a directory that is not a cycle workspace has no record")
	}
}
