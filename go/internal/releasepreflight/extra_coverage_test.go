package releasepreflight

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestExtractJSONVersion_Errors covers the two error branches: unreadable file
// and a file with no "version" field.
func TestExtractJSONVersion_Errors(t *testing.T) {
	t.Parallel()
	if _, err := ExtractJSONVersion(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("expected read error for missing file")
	}
	d := t.TempDir()
	p := filepath.Join(d, "plugin.json")
	if err := os.WriteFile(p, []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractJSONVersion(p); err == nil {
		t.Error("expected error for JSON with no version field")
	}
}

// TestDefaultGitClean_NonRepo covers the git-error branch: a non-repo dir makes
// `git diff --quiet HEAD` exit 128 (not the dirty-tree exit 1).
func TestDefaultGitClean_NonRepo(t *testing.T) {
	t.Parallel()
	clean, err := defaultGitClean(t.TempDir())
	if err == nil {
		t.Errorf("expected error for non-repo dir, got clean=%v err=nil", clean)
	}
}

// TestDefaultCurrentBranch_NonRepo covers the symbolic-ref error branch: a
// non-repo dir returns ("", nil) to mirror the bash detached-HEAD semantics.
func TestDefaultCurrentBranch_NonRepo(t *testing.T) {
	t.Parallel()
	branch, err := defaultCurrentBranch(t.TempDir())
	if err != nil {
		t.Errorf("non-repo dir should return nil error, got %v", err)
	}
	if branch != "" {
		t.Errorf("non-repo dir should return empty branch, got %q", branch)
	}
}

// TestDefaultGateTestRunner_Error covers the error branch without a nested
// `go test`: pointing at a non-existent module dir makes exec fail to start.
func TestDefaultGateTestRunner_Error(t *testing.T) {
	t.Parallel()
	// repoRoot/go does not exist → cmd.Dir chdir fails → CombinedOutput errors
	// before any `go test` subprocess is spawned.
	if err := defaultGateTestRunner(filepath.Join(t.TempDir(), "no-such-repo"), "./bogus"); err == nil {
		t.Error("expected error when the go module dir is absent")
	}
}

// TestDefaultSimulationRunner covers both branches via the EVOLVE_GO_BIN_TEST
// shim seam — a fake `go` that exits 0 (success) then 1 (failure), avoiding a
// real nested test run. Not parallel: mutates process env via t.Setenv.
func TestDefaultSimulationRunner(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(dir, "fake-go")
	t.Setenv("EVOLVE_GO_BIN_TEST", shim)

	if err := os.WriteFile(shim, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := defaultSimulationRunner(dir); err != nil {
		t.Errorf("shim exit 0 should succeed, got %v", err)
	}

	if err := os.WriteFile(shim, []byte("#!/bin/sh\necho boom; exit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := defaultSimulationRunner(dir); err == nil {
		t.Error("shim exit 1 should return an error")
	}
}

// auditEntry builds a single ledger JSONL line for an auditor entry.
func auditEntry(artifactPath, ts string) string {
	line := `{"role":"auditor"`
	if artifactPath != "" {
		line += `,"artifact_path":"` + artifactPath + `"`
	}
	if ts != "" {
		line += `,"ts":"` + ts + `"`
	}
	return line + "}\n"
}

func writeLedger(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(p, []byte(strings.Join(lines, "")), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestCheckRecentAudit_AllPhantom covers the all-entries-phantom branch: every
// auditor entry points at a missing artifact, so phantomCount>0 and no
// candidate is found.
func TestCheckRecentAudit_AllPhantom(t *testing.T) {
	t.Parallel()
	ledger := writeLedger(t,
		auditEntry("", "2026-05-27T00:00:00Z"),                             // empty path → phantom
		auditEntry("/nonexistent/audit-report.md", "2026-05-27T00:00:00Z"), // missing → phantom
	)
	_, err := checkRecentAudit(ledger, false, time.Now())
	if err == nil {
		t.Error("expected all-phantom error")
	}
}

// TestCheckRecentAudit_UnreadableArtifact covers the read-artifact error
// branch: the artifact path resolves (Stat ok) but is a directory, so ReadFile
// fails.
func TestCheckRecentAudit_UnreadableArtifact(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	artDir := filepath.Join(dir, "audit-report.md") // a directory, not a file
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(artDir, "2026-05-27T00:00:00Z"))
	_, err := checkRecentAudit(ledger, false, time.Now())
	if err == nil {
		t.Error("expected read error when artifact is a directory")
	}
}

// TestCheckRecentAudit_MissingTS covers the ts-missing branch: a valid PASS
// artifact but the ledger entry has no ts field.
func TestCheckRecentAudit_MissingTS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("Verdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, "")) // no ts
	_, err := checkRecentAudit(ledger, false, time.Now())
	if err == nil {
		t.Error("expected 'ledger entry missing ts' error")
	}
}

// TestCheckRecentAudit_UnparseableTS covers the ts-parse-fallback branch: an
// unparseable ts skips the age check and returns ok (nil error).
func TestCheckRecentAudit_UnparseableTS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	art := filepath.Join(dir, "audit-report.md")
	if err := os.WriteFile(art, []byte("Verdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger := writeLedger(t, auditEntry(art, "not-a-date"))
	res, err := checkRecentAudit(ledger, false, time.Now())
	if err != nil {
		t.Errorf("unparseable ts should skip age check (nil err), got %v", err)
	}
	if res.verdict != "PASS" {
		t.Errorf("verdict = %q, want PASS", res.verdict)
	}
}
