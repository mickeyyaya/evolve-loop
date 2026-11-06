package cycleoutcome

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
)

func writeLaneItem(t *testing.T, inbox, id, file string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"id": id, "title": "item " + id, "kind": "bug", "files": []string{file}})
	path := filepath.Join(inbox, "2026-11-05T00-00-00Z-"+id+".json")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestJourney_AForgedLanePinChangesNoItemAndNoReader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	inbox := filepath.Join(evolveDir, "inbox")
	writeLaneItem(t, inbox, "a", "go/internal/alpha/a.go")
	pathB := writeLaneItem(t, inbox, "b", "go/internal/beta/b.go")
	pathC := writeLaneItem(t, inbox, "c", "go/internal/gamma/c.go")
	rawB, _ := os.ReadFile(pathB)
	rawC, _ := os.ReadFile(pathC)
	lane := filepath.Join(evolveDir, "runs", "cycle-8")
	peer := filepath.Join(evolveDir, "runs", "cycle-9")
	for ws, ids := range map[string][]string{lane: {"a"}, peer: {"p"}} {
		if err := os.MkdirAll(ws, 0o755); err != nil {
			t.Fatal(err)
		}
		pin, _ := json.Marshal(committedset.LanePinDoc{TodoIDs: ids, GoalHash: "goal"})
		if err := os.WriteFile(filepath.Join(ws, "lane-scope.json"), pin, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opts := inboxmover.Options{ProjectRoot: root, Stderr: io.Discard}
	if _, err := inboxmover.ClaimDispatchable(opts, "a", "8"); err != nil {
		t.Fatalf("the real claim of the lane item: %v", err)
	}
	for cycle, ids := range map[int][]string{8: {"a"}, 9: {"p"}} {
		if err := inputindex.WriteRecord(root, inputindex.Record{Cycle: cycle, ItemIDs: ids, GoalHash: "goal"}); err != nil {
			t.Fatal(err)
		}
	}
	forged, _ := json.Marshal(committedset.LanePinDoc{TodoIDs: []string{"b", "c"}, GoalHash: "forged"})
	if err := os.WriteFile(filepath.Join(lane, "lane-scope.json"), forged, 0o644); err != nil {
		t.Fatal(err)
	}

	files := core.SiblingLaneFiles(evolveDir, 9, io.Discard)
	if !slices.Equal(files, []string{"go/internal/alpha/a.go"}) {
		t.Fatalf("the overlap read of the peer = %v, want only the file of item a", files)
	}

	if _, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: lane, Cycle: 8, Ceiling: 3, Stderr: io.Discard}); err != nil {
		t.Fatalf("the real closeout: %v", err)
	}

	for path, want := range map[string][]byte{pathB: rawB, pathC: rawC} {
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s changed after a closeout with a forged pin: %q %v", path, got, err)
		}
	}
	if got := itemFailureCount(t, findItemFile(t, inbox, "a")); got != 1 {
		t.Fatalf("the failure of the lane must charge item a once, got %d", got)
	}
	want := []string{"a"}
	committed, _ := committedset.Committed(lane)
	for name, got := range map[string][]string{"committedset.LanePin": committedset.LanePin(lane), "cycleoutcome.LaneScopeIDs": LaneScopeIDs(lane), "core.LaneScopeIDs": core.LaneScopeIDs(lane), "committedset.Committed": committed} {
		if !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
}
