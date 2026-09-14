package inboxmover

// extra_coverage_test.go — the readTaskIDOrUnknown fallbacks and the
// writeLedger tests moved to the lifecycle leaf with the code (ADR-0103 unit
// 06: lifecycle/item_test.go TestReadTaskIDOrUnknown_Fallbacks,
// lifecycle/ledger_test.go TestLedgerLine_NilLedgerIsSilent_AppendFailureIsTheVerbatimLine).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- findFileByTaskID: ReadDir error + skip-continue branches --------------

func TestFindFileByTaskID_ReadDirError(t *testing.T) {
	t.Parallel()
	if _, err := FindFileByTaskID(filepath.Join(t.TempDir(), "nope"), "x"); err == nil {
		t.Error("expected ReadDir error for missing dir")
	}
}

// TestFindFileByTaskID_SkipsUnreadableAndMalformed covers both per-file
// continue branches: an unreadable .json (ReadFile err) and a malformed .json
// (Unmarshal err) are skipped, then the valid match is found.
func TestFindFileByTaskID_SkipsUnreadableAndMalformed(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permissions")
	}
	dir := t.TempDir()
	// Names chosen so ReadDir (sorted) visits unreadable → malformed → good.
	unreadable := filepath.Join(dir, "a-unreadable.json")
	if err := os.WriteFile(unreadable, []byte(`{"id":"x"}`), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })
	if err := os.WriteFile(filepath.Join(dir, "b-malformed.json"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "c-good.json")
	if err := os.WriteFile(good, []byte(`{"id":"target"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindFileByTaskID(dir, "target")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != good {
		t.Errorf("got %q, want %q", got, good)
	}
}

// --- readActiveCycle: malformed JSON ---------------------------------------

func TestReadActiveCycle_MalformedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "cycle-state.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readActiveCycle(p); err == nil {
		t.Error("expected unmarshal error for malformed cycle-state.json")
	}
}

// --- Claim: mkdir + rename failure branches --------------------------------

func TestClaim_MkdirDestFails(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	dropInboxFile(t, repo, "task-1.json", "task-1")
	// Occupy processing/ with a file so the cycle dir cannot be created.
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "inbox", "processing"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Claim(Options{ProjectRoot: repo}, "task-1", "5")
	if !errors.Is(err, ErrMvFailed) {
		t.Errorf("err = %v, want ErrMvFailed", err)
	}
}

func TestClaim_RenameFails(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	dropInboxFile(t, repo, "task-1.json", "task-1")
	// Pre-create the destination path as a non-empty directory → rename fails.
	destDir := filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-5", "task-1.json")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "blocker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Claim(Options{ProjectRoot: repo}, "task-1", "5")
	if !errors.Is(err, ErrMvFailed) {
		t.Errorf("err = %v, want ErrMvFailed", err)
	}
}

// --- Promote: mkdir failure → loud error; rename failure → NoOp success -----

// TestPromote_MkdirFailsLoudly pins the inboxmover-promote-mkdir-fail-loud
// contract. This test previously asserted (NoOp=true, nil) — the ship.sh
// "source already moved" compat contract — for a destination mkdir failure,
// which is a genuine non-delivery: the item never moved and every caller read
// it as a completed promote. The assertion is INVERTED (not relaxed) on
// purpose: the failure must now surface as ErrMvFailed with NoOp false, while
// the promote-warn ledger line and the leave-the-file-alone behavior below stay
// exactly as they were.
func TestPromote_MkdirFailsLoudly(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	// Occupy processed/ with a file so processed/cycle-0 cannot be created.
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "inbox", "processed"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Promote(Options{ProjectRoot: repo}, "task-1", "processed", PromoteOpts{Cycle: "0"})
	if !errors.Is(err, ErrMvFailed) {
		t.Fatalf("err = %v, want ErrMvFailed: a destination mkdir failure is a non-delivery, not a no-op success", err)
	}
	if res.NoOp {
		t.Error("res.NoOp = true on mkdir failure: NoOp is the 'already moved' compat contract and must not cover a stranded task")
	}
	// The item must still be where it started — a loud error that also lost the
	// file would be worse than the silent no-op it replaces.
	if _, statErr := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-5", "task-1.json")); statErr != nil {
		t.Errorf("task-1.json left processing/cycle-5/ despite the failed promote: %v", statErr)
	}
	body, _ := os.ReadFile(filepath.Join(repo, ".evolve", "ledger.jsonl"))
	if !strings.Contains(string(body), `"action":"promote-warn"`) {
		t.Errorf("expected promote-warn ledger entry: %s", body)
	}
}

func TestPromote_RenameFailsNoOp(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	// retry dest = inbox/retry/task-1.json; pre-create as non-empty dir.
	dest := filepath.Join(repo, ".evolve", "inbox", "retry", "task-1.json")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "blocker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Promote(Options{ProjectRoot: repo}, "task-1", "retry", PromoteOpts{})
	if err != nil {
		t.Fatalf("promote should be NoOp success, got err %v", err)
	}
	if !res.NoOp {
		t.Error("expected NoOp on rename failure")
	}
}

// --- RecoverOrphans: rename failure → WARN continue ------------------------

func TestRecoverOrphans_RenameFails(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "3", "task-1.json", "task-1")
	// dest = inbox/task-1.json; pre-create as a non-empty dir → rename fails.
	dest := filepath.Join(repo, ".evolve", "inbox", "task-1.json")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "blocker"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := RecoverOrphans(Options{
		ProjectRoot:   repo,
		ActiveCycleFn: func() (string, error) { return "99", nil }, // cycle-3 is orphaned
	})
	if err != nil {
		t.Fatalf("recover should not error, got %v", err)
	}
	if res.Recovered != 0 {
		t.Errorf("rename failure should recover 0, got %d", res.Recovered)
	}
}
