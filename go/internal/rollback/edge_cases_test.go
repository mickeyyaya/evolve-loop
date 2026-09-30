package rollback

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadJournal_ReadError_NonExist(t *testing.T) {
	dir := t.TempDir()
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

func TestRun_AppendLedgerFailWarns(t *testing.T) {
	jp, repo := makeJournal(t, journalFull)

	blockerFile := filepath.Join(repo, "blocker")
	if err := os.WriteFile(blockerFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	badLedger := filepath.Join(blockerFile, "subdir", "ledger.jsonl")

	var buf strings.Builder
	sw := stringWriter{&buf}

	res, err := Run(Options{
		JournalPath: jp,
		RepoRoot:    repo,
		LedgerPath:  badLedger,
		Steps:       allOkSteps(),
		Stderr:      sw,
	})
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

type stringWriter struct{ b *strings.Builder }

func (sw stringWriter) Write(p []byte) (int, error) {
	return sw.b.Write(p)
}

func TestAppendLedger_OpenFileFails_ParentIsUnwritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping: running as root bypasses permission checks")
	}
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnly, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })

	ledgerPath := filepath.Join(readOnly, "ledger.jsonl")
	err := appendLedger(ledgerPath, []byte(`{"x":1}`))
	if err == nil {
		t.Error("expected error when parent dir is read-only")
	}
}

func TestResolveEvolveBinForRollback_PathLookup_Found(t *testing.T) {
	dir := t.TempDir()
	evolveBin := filepath.Join(dir, "evolve")
	if err := os.WriteFile(evolveBin, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", dir)

	repoRoot := t.TempDir()
	got := resolveEvolveBinForRollback(repoRoot)
	if got == "" {
		t.Error("expected non-empty path when evolve is in PATH")
	}
	if !strings.Contains(got, "evolve") {
		t.Errorf("got %q, expected path containing 'evolve'", got)
	}
}
