package inboxmover

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- readTaskIDOrUnknown: the three "unknown" fallbacks --------------------

func TestReadTaskIDOrUnknown_Fallbacks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if got := readTaskIDOrUnknown(filepath.Join(dir, "missing.json")); got != "unknown" {
		t.Errorf("missing file: got %q, want unknown", got)
	}

	malformed := filepath.Join(dir, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readTaskIDOrUnknown(malformed); got != "unknown" {
		t.Errorf("malformed: got %q, want unknown", got)
	}

	noID := filepath.Join(dir, "no-id.json")
	if err := os.WriteFile(noID, []byte(`{"payload":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readTaskIDOrUnknown(noID); got != "unknown" {
		t.Errorf("empty id: got %q, want unknown", got)
	}
}

// --- findFileByTaskID: ReadDir error + skip-continue branches --------------

func TestFindFileByTaskID_ReadDirError(t *testing.T) {
	t.Parallel()
	if _, err := findFileByTaskID(filepath.Join(t.TempDir(), "nope"), "x"); err == nil {
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
	got, err := findFileByTaskID(dir, "target")
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

// --- writeLedger: best-effort silent-drop branches -------------------------

func TestWriteLedger_MkdirFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// LedgerPath under a regular file → MkdirAll(dir) fails; writeLedger drops.
	ledger := filepath.Join(blocker, "ledger.jsonl")
	opts := Options{LedgerPath: ledger, Now: time.Now}
	writeLedger(opts, LedgerEntry{Action: "claim"}) // must not panic
	if _, err := os.Stat(ledger); err == nil {
		t.Error("ledger should not exist after mkdir failure")
	}
}

func TestWriteLedger_OpenFileFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// LedgerPath is itself a directory → OpenFile(O_WRONLY) fails; drops silently.
	ledgerAsDir := filepath.Join(dir, "ledger.jsonl")
	if err := os.MkdirAll(ledgerAsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	opts := Options{LedgerPath: ledgerAsDir, Now: time.Now}
	writeLedger(opts, LedgerEntry{Action: "claim"}) // must not panic
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

// --- Promote: mkdir + rename failure → NoOp success ------------------------

func TestPromote_MkdirFailsNoOp(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	dropProcessingFile(t, repo, "5", "task-1.json", "task-1")
	// Occupy processed/ with a file so processed/cycle-0 cannot be created.
	if err := os.WriteFile(filepath.Join(repo, ".evolve", "inbox", "processed"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Promote(Options{ProjectRoot: repo}, "task-1", "processed", PromoteOpts{Cycle: "0"})
	if err != nil {
		t.Fatalf("promote should be NoOp success, got err %v", err)
	}
	if !res.NoOp {
		t.Error("expected NoOp on mkdir failure")
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
