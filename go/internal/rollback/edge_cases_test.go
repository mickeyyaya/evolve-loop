// edge_cases_test.go — fast (no subprocess) behavior-probing tests for rollback.go.
//
// Coverage targets (fast tier):
//   - ReadJournal: read error (non-ENOENT), missing version, missing commit_sha, missing branch
//   - Run: appendLedger failure warning path
//   - appendLedger: OpenFile failure
//   - resolveEvolveBinForRollback: PATH-lookup branch (evolve in PATH)
//
// Subprocess-dependent tests (nil-step wiring, defaultGhDeleteRelease with fake gh,
// defaultDeleteRemoteTag with fake git, defaultRevertAndShip with fake git/evolve)
// live in edge_cases_integration_test.go behind //go:build integration.
package rollback

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ReadJournal edge cases
// ---------------------------------------------------------------------------

// TestReadJournal_ReadError_NonExist — os.ReadFile fails for a reason OTHER than
// ENOENT (e.g. permission denied). Expect ErrJournalMalformed (not ErrJournalNotFound).
func TestReadJournal_ReadError_NonExist(t *testing.T) {
	dir := t.TempDir()
	// Create a directory where the "file" should be — os.ReadFile on a directory
	// returns a non-ENOENT error on all platforms.
	dirAsFile := filepath.Join(dir, "not-a-file")
	if err := os.Mkdir(dirAsFile, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := ReadJournal(dirAsFile)
	if err == nil {
		t.Fatal("expected error reading a directory as a file")
	}
	if errors.Is(err, ErrJournalNotFound) {
		t.Errorf("expected ErrJournalMalformed (read-fail), got ErrJournalNotFound; err=%v", err)
	}
	if !errors.Is(err, ErrJournalMalformed) {
		t.Errorf("expected ErrJournalMalformed, got: %v", err)
	}
}

// TestReadJournal_MissingVersion — JSON is valid but 'version' field is empty.
func TestReadJournal_MissingVersion(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "j.json")
	if err := os.WriteFile(p, []byte(`{"tag":"v1.0.0","commit_sha":"abc","branch":"main"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadJournal(p)
	if !errors.Is(err, ErrJournalMalformed) {
		t.Fatalf("err = %v, want ErrJournalMalformed", err)
	}
	if !strings.Contains(err.Error(), "version") {
		t.Errorf("err = %v, want mention of 'version'", err)
	}
}

// TestReadJournal_MissingCommitSHA — version + tag present but commit_sha missing.
func TestReadJournal_MissingCommitSHA(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "j.json")
	if err := os.WriteFile(p, []byte(`{"version":"1.0.0","tag":"v1.0.0","branch":"main"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadJournal(p)
	if !errors.Is(err, ErrJournalMalformed) {
		t.Fatalf("err = %v, want ErrJournalMalformed", err)
	}
	if !strings.Contains(err.Error(), "commit_sha") {
		t.Errorf("err = %v, want mention of 'commit_sha'", err)
	}
}

// TestReadJournal_MissingBranch — version + tag + commit_sha present but branch missing.
func TestReadJournal_MissingBranch(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "j.json")
	if err := os.WriteFile(p, []byte(`{"version":"1.0.0","tag":"v1.0.0","commit_sha":"abc123"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadJournal(p)
	if !errors.Is(err, ErrJournalMalformed) {
		t.Fatalf("err = %v, want ErrJournalMalformed", err)
	}
	if !strings.Contains(err.Error(), "branch") {
		t.Errorf("err = %v, want mention of 'branch'", err)
	}
}

// ---------------------------------------------------------------------------
// Run: appendLedger failure warning (WARN path)
// ---------------------------------------------------------------------------

// TestRun_AppendLedgerFailWarns — when the ledger path is unwritable, Run logs
// "WARN:" but still returns the step results (not an error). This covers the
// `logf("WARN: failed to append rollback ledger: ...")` branch.
func TestRun_AppendLedgerFailWarns(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)

	// Point ledger at a path whose parent is a FILE (not a directory), so
	// appendLedger's MkdirAll fails.
	blockerDir := filepath.Join(repo, "blocker")
	if err := os.WriteFile(blockerDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	badLedger := filepath.Join(blockerDir, "subdir", "ledger.jsonl")

	var buf strings.Builder
	sw := stringWriter{&buf}

	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		LedgerPath:  badLedger,
		Steps:       allOkSteps(),
		Stderr:      sw,
	})
	// Run should succeed (revert=reverted, no failed steps) even when ledger write fails.
	if err != nil {
		t.Fatalf("Run err = %v, want nil (ledger failure is non-fatal)", err)
	}
	if !res.OverallSucceeded {
		t.Error("OverallSucceeded should be true when steps succeed (ledger write is best-effort)")
	}
	if !strings.Contains(buf.String(), "WARN") {
		t.Errorf("expected WARN log for ledger failure; log = %s", buf.String())
	}
}

// stringWriter bridges strings.Builder to io.Writer for Stderr.
type stringWriter struct{ b *strings.Builder }

func (sw stringWriter) Write(p []byte) (int, error) {
	return sw.b.Write(p)
}

// ---------------------------------------------------------------------------
// appendLedger: OpenFile failure
// ---------------------------------------------------------------------------

// TestAppendLedger_OpenFileFails_ParentIsUnwritable — MkdirAll succeeds but the
// resulting directory has no write permission, so OpenFile fails.
//
// NOTE: This test is skipped when running as root (root ignores mode bits).
func TestAppendLedger_OpenFileFails_ParentIsUnwritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root bypasses permission checks")
	}
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnly, 0o755); err != nil {
		t.Fatal(err)
	}
	// Remove write permission so OpenFile inside will fail.
	if err := os.Chmod(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) }) // restore for cleanup

	ledgerPath := filepath.Join(readOnly, "ledger.jsonl")
	err := appendLedger(ledgerPath, []byte(`{"x":1}`))
	if err == nil {
		t.Error("expected error when parent dir is read-only")
	}
}

// ---------------------------------------------------------------------------
// resolveEvolveBinForRollback: PATH lookup branch
// ---------------------------------------------------------------------------

// TestResolveEvolveBinForRollback_PathLookup_Found — when EVOLVE_GO_BIN is unset
// and no <repoRoot>/go/bin/evolve exists, but 'evolve' is in PATH, it should
// return the PATH-resolved path.
func TestResolveEvolveBinForRollback_PathLookup_Found(t *testing.T) {
	dir := t.TempDir()
	// Place a fake evolve in a temp dir and add it to PATH.
	evolveBin := filepath.Join(dir, "evolve")
	if err := os.WriteFile(evolveBin, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", dir) // only this dir in PATH

	// Use a repoRoot that has no go/bin/evolve.
	repoRoot := t.TempDir()
	got := resolveEvolveBinForRollback(repoRoot)
	if got == "" {
		t.Error("expected non-empty path when evolve is in PATH")
	}
	if !strings.Contains(got, "evolve") {
		t.Errorf("got %q, expected path containing 'evolve'", got)
	}
}
