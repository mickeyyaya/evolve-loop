package cycleoutcome

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

func decisionWorkspace(t *testing.T, body string) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestFailedCommitted_NoPinKeepsTopNThenSkipShipped(t *testing.T) {
	t.Parallel()
	ws := decisionWorkspace(t, `{"top_n":[{"id":"one"},{"id":"two"}],"skip_shipped":[{"task_id":"three"},{"task_id":"one"}],"deferred":[{"id":"menu"}],"dropped":[{"id":"gone"}]}`)
	if got := failedCommitted(ws); !slices.Equal(got, []string{"one", "two", "three"}) {
		t.Fatalf("failedCommitted = %v, want [one two three]", got)
	}
}

func TestFailedCommitted_NoPinLeavesOutADeferredIDOfTopN(t *testing.T) {
	t.Parallel()
	ws := decisionWorkspace(t, `{"top_n":[{"id":"a"},{"id":"c"}],"deferred":[{"id":"a"}]}`)
	if got := failedCommitted(ws); !slices.Equal(got, []string{"c"}) {
		t.Fatalf("failedCommitted = %v, want [c]", got)
	}
}

func TestFailedCommitted_APinIsTheUpperBoundOfTheCharge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 7, ItemIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(`{"top_n":[{"id":"a"},{"id":"x"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := failedCommitted(ws); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("failedCommitted = %v, want [a]: top_n cannot widen the charge past the pin", got)
	}
}

func TestFailedCommitted_APinWinsOverTheDecision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-4")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 4, ItemIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	body := `{"top_n":[{"id":"b"}],"skip_shipped":[{"task_id":"c"}]}`
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := failedCommitted(ws); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("failedCommitted = %v, want [a]", got)
	}
}

func TestFailedCommitted_AbsentCorruptOrEmptyDecisionIsEmpty(t *testing.T) {
	t.Parallel()
	for name, ws := range map[string]string{"absent": t.TempDir(), "corrupt": decisionWorkspace(t, "not json{"), "empty": decisionWorkspace(t, `{"top_n":[]}`)} {
		if got := failedCommitted(ws); len(got) != 0 {
			t.Errorf("%s: failedCommitted = %v, want empty", name, got)
		}
	}
}

func TestShippedIDsIgnoresBlankIDs(t *testing.T) {
	t.Parallel()
	ws := decisionWorkspace(t, `{"skip_shipped":[{"task_id":" "},{"task_id":"x"}]}`)
	if got := committedset.ShippedIDs(ws); !slices.Equal(got, []string{"x"}) {
		t.Fatalf("ShippedIDs = %v", got)
	}
}

func TestFailedCommitted_APinDropsTheItemsTriageAnswered(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-5")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 5, ItemIDs: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	body := `{"top_n":[],"dropped":[{"id":"a","reason":"landed"}]}`
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := failedCommitted(ws); !slices.Equal(got, []string{"b"}) {
		t.Fatalf("failedCommitted = %v, want [b]", got)
	}
}
