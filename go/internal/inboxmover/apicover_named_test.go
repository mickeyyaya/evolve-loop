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
