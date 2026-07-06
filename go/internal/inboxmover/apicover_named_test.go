package inboxmover

import (
	"os"
	"path/filepath"
	"testing"
)

func seedInbox(t *testing.T, taskID string) (root, inboxDir string) {
	t.Helper()
	root = t.TempDir()
	inboxDir = filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, taskID+".json"), []byte(`{"id":"`+taskID+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, inboxDir
}

// TestClaimResult_NamesAndMovesToProcessing names the ClaimResult type and pins
// that Claim records the src/dest paths AND physically relocates the task into
// processing/cycle-N/.
func TestClaimResult_NamesAndMovesToProcessing(t *testing.T) {
	root, inboxDir := seedInbox(t, "task-a")

	var res ClaimResult
	res, err := Claim(Options{ProjectRoot: root}, "task-a", "42")
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if res.SrcPath != filepath.Join(inboxDir, "task-a.json") {
		t.Errorf("ClaimResult.SrcPath=%q, want the inbox file", res.SrcPath)
	}
	wantDest := filepath.Join(inboxDir, "processing", "cycle-42", "task-a.json")
	if res.DestPath != wantDest {
		t.Errorf("ClaimResult.DestPath=%q, want %q", res.DestPath, wantDest)
	}
	if _, err := os.Stat(res.SrcPath); !os.IsNotExist(err) {
		t.Errorf("source must be gone after claim; stat err=%v", err)
	}
	if _, err := os.Stat(res.DestPath); err != nil {
		t.Errorf("dest must exist after claim; stat err=%v", err)
	}
}

// TestPromoteResult_NamesNoOpWhenMissing names PromoteResult and pins the
// ship.sh-compat contract: promoting a missing task is a no-op (NoOp=true, no
// error, empty paths) rather than a failure.
func TestPromoteResult_NamesNoOpWhenMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}

	var res PromoteResult
	res, err := Promote(Options{ProjectRoot: root}, "ghost", "processed", PromoteOpts{})
	if err != nil {
		t.Fatalf("Promote of a missing task must not error (ship.sh compat); got %v", err)
	}
	if !res.NoOp {
		t.Errorf("PromoteResult.NoOp=false, want true for a missing task")
	}
	if res.SrcPath != "" || res.DestPath != "" {
		t.Errorf("PromoteResult paths should be empty on NoOp; got src=%q dest=%q", res.SrcPath, res.DestPath)
	}
}

// TestSupersededInboxIDs_NamesDedupAndTolerance names SupersededInboxIDs and
// pins that it dedups/order-preserves the top-level "superseded" array and
// returns empty (never panics) on an absent field or invalid JSON.
func TestSupersededInboxIDs_NamesDedupAndTolerance(t *testing.T) {
	got := SupersededInboxIDs([]byte(`{"superseded":["a","b","a"]}`))
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("SupersededInboxIDs=%v, want [a b] (deduped, order-preserving)", got)
	}
	if n := len(SupersededInboxIDs([]byte(`{"top_n":[]}`))); n != 0 {
		t.Errorf("absent field: got %d ids, want 0", n)
	}
	if n := len(SupersededInboxIDs([]byte(`}{not json`))); n != 0 {
		t.Errorf("invalid JSON: got %d ids, want 0 (must tolerate, never panic)", n)
	}
}

// TestReconcileSuperseded_NamesRetireByIDAndNoOp names ReconcileSuperseded and
// pins its two load-bearing properties: a present id is retired by id alone
// (file leaves the inbox root), and an absent id is a clean idempotent no-op.
func TestReconcileSuperseded_NamesRetireByIDAndNoOp(t *testing.T) {
	root, inboxDir := seedInbox(t, "orphan-x")

	retired, err := ReconcileSuperseded(Options{ProjectRoot: root}, []string{"orphan-x"}, "processed", PromoteOpts{Cycle: "9"})
	if err != nil {
		t.Fatalf("ReconcileSuperseded: %v", err)
	}
	if len(retired) != 1 || retired[0] != "orphan-x" {
		t.Fatalf("retired=%v, want [orphan-x]", retired)
	}
	if _, statErr := os.Stat(filepath.Join(inboxDir, "orphan-x.json")); !os.IsNotExist(statErr) {
		t.Errorf("retired item must leave the inbox root; stat err=%v", statErr)
	}

	noop, err := ReconcileSuperseded(Options{ProjectRoot: root}, []string{"never-here"}, "processed", PromoteOpts{Cycle: "9"})
	if err != nil {
		t.Fatalf("absent id must be a clean no-op, got err=%v", err)
	}
	if len(noop) != 0 {
		t.Errorf("absent id retired %v, want none", noop)
	}
}

// TestRecoverResult_NamesAndCountsOrphans names RecoverResult and pins that an
// orphan under an inactive cycle is recovered back to the inbox root and counted.
func TestRecoverResult_NamesAndCountsOrphans(t *testing.T) {
	root := t.TempDir()
	inboxDir := filepath.Join(root, ".evolve", "inbox")
	orphanDir := filepath.Join(inboxDir, "processing", "cycle-7")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphanDir, "stuck.json"), []byte(`{"id":"stuck"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var res RecoverResult
	res, err := RecoverOrphans(Options{
		ProjectRoot:   root,
		ActiveCycleFn: func() (string, error) { return "99", nil },
	})
	if err != nil {
		t.Fatalf("RecoverOrphans: %v", err)
	}
	if res.Recovered != 1 {
		t.Errorf("RecoverResult.Recovered=%d, want 1", res.Recovered)
	}
	wantDest := filepath.Join(inboxDir, "stuck.json")
	if len(res.Paths) != 1 || res.Paths[0] != wantDest {
		t.Errorf("RecoverResult.Paths=%v, want [%s]", res.Paths, wantDest)
	}
	if _, err := os.Stat(wantDest); err != nil {
		t.Errorf("recovered file must be at inbox root; stat err=%v", err)
	}
}
