package rollback

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const journalFull = `{
  "version": "1.2.3",
  "tag": "v1.2.3",
  "commit_sha": "abcdef1234567890",
  "branch": "main",
  "release_url": "https://github.com/example/repo/releases/tag/v1.2.3",
  "started_at": "2026-04-27T08:00:00Z",
  "completed_at": "2026-04-27T08:05:00Z"
}`

// makeJournal writes a journal JSON file in a temp repo and returns its path
// plus the repoRoot.
func makeJournal(t *testing.T, journalJSON string) (journalPath, repoRoot string) {
	t.Helper()
	repoRoot = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".evolve"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	journalPath = filepath.Join(repoRoot, ".evolve", "journal.json")
	if journalJSON != "" {
		if err := os.WriteFile(journalPath, []byte(journalJSON), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return journalPath, repoRoot
}

// allOkSteps returns Steps that simulate the happy path.
func allOkSteps() Steps {
	return Steps{
		GhDeleteRelease: func(string) string { return "deleted" },
		DeleteRemoteTag: func(string, string) string { return "deleted" },
		RevertAndShip:   func(string, string, string, string) string { return "reverted" },
	}
}

// === Test 1: dry-run with valid journal → exit 0, no ledger write ===========
func TestRun_DryRun(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	var buf bytes.Buffer
	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		DryRun:      true,
		Stderr:      &buf,
	})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}
	if !res.OverallSucceeded {
		t.Error("want OverallSucceeded=true")
	}
	if !strings.Contains(buf.String(), "DRY-RUN") {
		t.Errorf("log missing DRY-RUN: %s", buf.String())
	}
	ledgerPath := filepath.Join(repo, ".evolve", "release-rollbacks.jsonl")
	if _, err := os.Stat(ledgerPath); err == nil {
		t.Error("dry-run should NOT write the ledger file")
	}
}

// === Test 2: malformed journal (missing 'tag') → ErrJournalMalformed =======
func TestRun_MissingTag(t *testing.T) {
	jp, repo := makeJournal(t, `{"version":"1.0.0","commit_sha":"abc","branch":"main"}`)
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		DryRun:      true,
	})
	if !errors.Is(err, ErrJournalMalformed) {
		t.Fatalf("err = %v, want ErrJournalMalformed", err)
	}
	if !strings.Contains(err.Error(), "tag") {
		t.Errorf("err = %v, want contains 'tag'", err)
	}
}

// === Test 3: nonexistent journal → ErrJournalNotFound =======================
func TestRun_MissingJournal(t *testing.T) {
	_, repo := makeJournal(t, "")
	_, err := Run(Options{
		JournalPath: filepath.Join(repo, ".evolve", "missing.json"),
		RepoRoot:    repo,
		DryRun:      true,
	})
	if !errors.Is(err, ErrJournalNotFound) {
		t.Fatalf("err = %v, want ErrJournalNotFound", err)
	}
}

// === Test 4: --reason captured in ledger entry ==============================
func TestRun_ReasonInLedger(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Reason:      "audit fail probe",
		Steps:       allOkSteps(),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(res.LedgerEntryJSON, "audit fail probe") {
		t.Errorf("LedgerEntryJSON missing reason: %s", res.LedgerEntryJSON)
	}
	// On-disk ledger.
	body, err := os.ReadFile(filepath.Join(repo, ".evolve", "release-rollbacks.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if !strings.Contains(string(body), "audit fail probe") {
		t.Errorf("ledger file missing reason: %s", body)
	}
}

// === Test 5: missing journal path argument → cmd layer covers exit 10 ======
// (programmer-error case: covered at cmd test layer)

// === Test 6: ledger has required fields =====================================
func TestRun_LedgerSchema(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Reason:      "structural test",
		Steps:       allOkSteps(),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	body, err := os.ReadFile(filepath.Join(repo, ".evolve", "release-rollbacks.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(body), &entry); err != nil {
		t.Fatalf("ledger not valid JSON: %v\n%s", err, body)
	}
	for _, field := range []string{
		"version", "tag", "commit_sha", "reason",
		"release_delete", "tag_delete", "revert",
	} {
		if _, ok := entry[field]; !ok {
			t.Errorf("ledger entry missing field %q: %v", field, entry)
		}
	}
}

// === Test 7: RevertAndShip seam receives the expected sha/reason/version ===
func TestRun_RevertAndShipReceivesCorrectArgs(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	var gotSha, gotReason, gotVersion string
	steps := allOkSteps()
	steps.RevertAndShip = func(_, sha, reason, version string) string {
		gotSha, gotReason, gotVersion = sha, reason, version
		return "reverted"
	}
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Reason:      "bypass test",
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if gotSha != "abcdef1234567890" {
		t.Errorf("sha = %q, want abcdef1234567890", gotSha)
	}
	if gotReason != "bypass test" {
		t.Errorf("reason = %q, want 'bypass test'", gotReason)
	}
	if gotVersion != "1.2.3" {
		t.Errorf("version = %q, want '1.2.3'", gotVersion)
	}
}

// === Test 8: MEDIUM-1 REGRESSION — partial rollback → ErrPartial ===========
// gh delete fails, revert succeeds → must NOT report overall success.
func TestRun_PartialRollback_GhFailRevertOK(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	steps := Steps{
		GhDeleteRelease: func(string) string { return "failed" }, // step 1 fails
		DeleteRemoteTag: func(string, string) string { return "not-present" },
		RevertAndShip:   func(string, string, string, string) string { return "reverted" },
	}
	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Reason:      "MEDIUM-1 regression",
		Steps:       steps,
	})
	if !errors.Is(err, ErrPartial) {
		t.Fatalf("err = %v, want ErrPartial", err)
	}
	if res.OverallSucceeded {
		t.Error("OverallSucceeded must be false on partial rollback")
	}
	if res.ReleaseDelete != "failed" {
		t.Errorf("ReleaseDelete = %q, want failed", res.ReleaseDelete)
	}
	if res.Revert != "reverted" {
		t.Errorf("Revert = %q, want reverted", res.Revert)
	}
}

// === Step 2 failure also blocks overall success ============================
func TestRun_PartialRollback_TagFail(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	steps := Steps{
		GhDeleteRelease: func(string) string { return "deleted" },
		DeleteRemoteTag: func(string, string) string { return "failed" },
		RevertAndShip:   func(string, string, string, string) string { return "reverted" },
	}
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Reason:      "tag fail",
		Steps:       steps,
	})
	if !errors.Is(err, ErrPartial) {
		t.Fatalf("err = %v, want ErrPartial", err)
	}
}

// === Step 3 failure (revert failed entirely) → ErrPartial ==================
func TestRun_PartialRollback_RevertFail(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	steps := Steps{
		GhDeleteRelease: func(string) string { return "deleted" },
		DeleteRemoteTag: func(string, string) string { return "deleted" },
		RevertAndShip:   func(string, string, string, string) string { return "failed" },
	}
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps:       steps,
	})
	if !errors.Is(err, ErrPartial) {
		t.Fatalf("err = %v, want ErrPartial", err)
	}
}

// === local-only revert (push failed) → ErrPartial ==========================
func TestRun_PartialRollback_LocalOnly(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	steps := Steps{
		GhDeleteRelease: func(string) string { return "deleted" },
		DeleteRemoteTag: func(string, string) string { return "deleted" },
		RevertAndShip:   func(string, string, string, string) string { return "local-only" },
	}
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps:       steps,
	})
	if !errors.Is(err, ErrPartial) {
		t.Fatalf("err = %v, want ErrPartial (local-only push failure)", err)
	}
}

// === Skipped statuses (not-present, skipped) do NOT block overall success ==
func TestRun_SkippedStatuses_StillSuccess(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	steps := Steps{
		GhDeleteRelease: func(string) string { return "skipped" }, // no gh
		DeleteRemoteTag: func(string, string) string { return "not-present" },
		RevertAndShip:   func(string, string, string, string) string { return "reverted" },
	}
	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !res.OverallSucceeded {
		t.Errorf("OverallSucceeded = false, want true (skipped/not-present are legitimate)")
	}
}

// === ReadJournal happy path + malformed JSON ===============================
func TestReadJournal_Happy(t *testing.T) {
	jp, _ := makeJournal(t, journalFull)
	j, err := ReadJournal(jp)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if j.Version != "1.2.3" || j.Tag != "v1.2.3" {
		t.Errorf("Journal = %+v", j)
	}
}

func TestReadJournal_NotFound(t *testing.T) {
	_, err := ReadJournal("/tmp/this-file-definitely-does-not-exist-rollback-test-xyz.json")
	if !errors.Is(err, ErrJournalNotFound) {
		t.Errorf("err = %v, want ErrJournalNotFound", err)
	}
}

func TestReadJournal_InvalidJSON(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "bad.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ReadJournal(p)
	if !errors.Is(err, ErrJournalMalformed) {
		t.Errorf("err = %v, want ErrJournalMalformed", err)
	}
}

// === appendLedger creates parent dir + appends NDJSON line =================
func TestAppendLedger_CreatesDir(t *testing.T) {
	d := t.TempDir()
	path := filepath.Join(d, "nested", "deeper", "ledger.jsonl")
	if err := appendLedger(path, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("append: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(body) != "{\"a\":1}\n" {
		t.Errorf("ledger body = %q", body)
	}
	// Append second line.
	if err := appendLedger(path, []byte(`{"b":2}`)); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	body, _ = os.ReadFile(path)
	if string(body) != "{\"a\":1}\n{\"b\":2}\n" {
		t.Errorf("ledger after 2 appends = %q", body)
	}
}

// === Default step impls: at-least-callable smoke tests =====================
// Real gh/git invocations are out of scope (they require a remote + auth),
// but we exercise the no-op early returns:
//   - defaultGhDeleteRelease without `gh` in PATH → "skipped"
//   - defaultDeleteRemoteTag without remote tag → "not-present"
//   - defaultRevertAndShip on a fresh non-git dir → "failed" (no commit to revert)
func TestDefaultGhDeleteRelease_NoGh(t *testing.T) {
	t.Setenv("PATH", "/nonexistent-bin-dir-for-rollback-test")
	if got := defaultGhDeleteRelease("v0.0.0-nope"); got != "skipped" {
		t.Errorf("got %q, want 'skipped' when gh not in PATH", got)
	}
}

func TestDefaultDeleteRemoteTag_NonGitDir(t *testing.T) {
	d := t.TempDir() // not a git repo
	// ls-remote will fail → empty output → treated as "not-present".
	if got := defaultDeleteRemoteTag(d, "v0.0.0-nope"); got != "not-present" {
		t.Errorf("got %q, want 'not-present' on non-git dir", got)
	}
}

func TestDefaultRevertAndShip_NonGitDir(t *testing.T) {
	d := t.TempDir()
	if got := defaultRevertAndShip(d, "deadbeef", "x", "0.0.0"); got != "failed" {
		t.Errorf("got %q, want 'failed' on non-git dir", got)
	}
}

func TestDefaultSteps_WiresAllThree(t *testing.T) {
	s := DefaultSteps()
	if s.GhDeleteRelease == nil || s.DeleteRemoteTag == nil || s.RevertAndShip == nil {
		t.Errorf("DefaultSteps missing implementations: %+v", s)
	}
}

// === Run honors LedgerPath override =========================================
func TestRun_LedgerPathOverride(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	customLedger := filepath.Join(repo, "custom", "ledger.jsonl")
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		LedgerPath:  customLedger,
		Steps:       allOkSteps(),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(customLedger); err != nil {
		t.Errorf("custom ledger not written at %s", customLedger)
	}
	// Default ledger location should NOT exist.
	defaultPath := filepath.Join(repo, ".evolve", "release-rollbacks.jsonl")
	if _, err := os.Stat(defaultPath); err == nil {
		t.Errorf("default ledger should not exist when LedgerPath override given")
	}
}

// === Empty Reason defaults to "release-pipeline failure" ===================
func TestRun_DefaultReason(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Steps:       allOkSteps(),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if res.Reason != "release-pipeline failure" {
		t.Errorf("Reason = %q, want default 'release-pipeline failure'", res.Reason)
	}
}

// === Empty JournalPath → ErrJournalNotFound (no path resolution attempted) =
func TestRun_EmptyJournalPath(t *testing.T) {
	_, err := Run(Options{})
	if !errors.Is(err, ErrJournalNotFound) {
		t.Errorf("err = %v, want ErrJournalNotFound", err)
	}
}

// === Now seam reflects in ledger timestamp =================================
func TestRun_NowSeamUsed(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)
	fixed := time.Date(2026, 1, 15, 12, 30, 45, 0, time.UTC)
	_, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		Now:         func() time.Time { return fixed },
		Steps:       allOkSteps(),
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(repo, ".evolve", "release-rollbacks.jsonl"))
	if !strings.Contains(string(body), "2026-01-15T12:30:45Z") {
		t.Errorf("ledger missing fixed timestamp: %s", body)
	}
}
