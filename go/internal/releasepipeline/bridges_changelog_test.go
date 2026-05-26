package releasepipeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// makeHermeticGitRepo creates a minimal hermetic git repository in a TempDir
// with one commit tagged v0.0.1, suitable for testing git-dependent code paths
// without depending on the operator's real repo state.
//
// Returns the repo root path.
func makeHermeticGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH — skip hermetic git tests")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	// Write a README and create the initial commit.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-q", "-m", "feat: initial commit")
	run("tag", "v0.0.1")
	return dir
}

// === runChangelogGenLib — idempotent skip when entry already exists ===========

// TestRunChangelogGenLib_IdempotentSkip: when CHANGELOG.md already has the
// [target] entry, runChangelogGenLib returns nil without calling WriteEntry.
// We verify this by using a CHANGELOG that already contains the entry and
// confirming no error is returned.
func TestRunChangelogGenLib_IdempotentSkip(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	// Write a CHANGELOG with an existing entry for "1.0.0".
	clBody := "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n### Added\n- existing\n"
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(clBody), 0o644); err != nil {
		t.Fatalf("write CHANGELOG: %v", err)
	}

	err := runChangelogGenLib(dir, "v0.0.1", "HEAD", "1.0.0", false)
	if err != nil {
		t.Errorf("runChangelogGenLib idempotent skip: want nil, got %v", err)
	}

	// Verify the CHANGELOG is unchanged (WriteEntry was not called).
	body, _ := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if string(body) != clBody {
		t.Errorf("CHANGELOG was modified (expected idempotent skip)\ngot: %s", string(body))
	}
}

// TestRunChangelogGenLib_InvalidSemver: a non-semver target returns an error
// immediately, before any git or file operations.
func TestRunChangelogGenLib_InvalidSemver(t *testing.T) {
	err := runChangelogGenLib(t.TempDir(), "v0.0.1", "HEAD", "not-semver", false)
	if err == nil {
		t.Fatal("runChangelogGenLib with invalid semver: want error, got nil")
	}
	if !containsStr(err.Error(), "not semver") {
		t.Errorf("error = %q, want mention of 'not semver'", err.Error())
	}
}

// TestRunChangelogGenLib_VerifyFromRefFails: when fromRef does not exist in
// the repo, VerifyRef returns an error and the function propagates it.
func TestRunChangelogGenLib_VerifyFromRefFails(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	err := runChangelogGenLib(dir, "nonexistent-tag", "HEAD", "2.0.0", false)
	if err == nil {
		t.Fatal("runChangelogGenLib with bad fromRef: want error, got nil")
	}
}

// TestRunChangelogGenLib_VerifyToRefFails: when toRef does not exist in the
// repo, VerifyRef for toRef returns an error and the function propagates it.
func TestRunChangelogGenLib_VerifyToRefFails(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	// fromRef=v0.0.1 is valid, but toRef is invalid.
	err := runChangelogGenLib(dir, "v0.0.1", "nonexistent-branch", "2.0.0", false)
	if err == nil {
		t.Fatal("runChangelogGenLib with bad toRef: want error, got nil")
	}
}

// TestRunChangelogGenLib_DryRun: when dryRun=true, no CHANGELOG write occurs
// and the function returns nil.
func TestRunChangelogGenLib_DryRun(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	// No CHANGELOG.md exists, so HasEntry check is skipped.
	err := runChangelogGenLib(dir, "v0.0.1", "HEAD", "2.0.0", true /*dryRun*/)
	if err != nil {
		t.Errorf("runChangelogGenLib dry-run: want nil, got %v", err)
	}

	// CHANGELOG.md must NOT have been created.
	if _, err2 := os.Stat(filepath.Join(dir, "CHANGELOG.md")); err2 == nil {
		t.Error("dry-run must NOT create CHANGELOG.md")
	}
}

// TestRunChangelogGenLib_LiveWrite: when dryRun=false and the repo has valid
// refs and no prior CHANGELOG entry, WriteEntry creates/updates CHANGELOG.md.
func TestRunChangelogGenLib_LiveWrite(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	err := runChangelogGenLib(dir, "v0.0.1", "HEAD", "2.0.0", false /*dryRun*/)
	if err != nil {
		t.Fatalf("runChangelogGenLib live write: %v", err)
	}

	// CHANGELOG.md must exist and contain the [2.0.0] entry.
	body, readErr := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if readErr != nil {
		t.Fatalf("CHANGELOG.md not created: %v", readErr)
	}
	if !containsStr(string(body), "[2.0.0]") {
		t.Errorf("CHANGELOG.md does not contain [2.0.0] entry:\n%s", string(body))
	}
}
