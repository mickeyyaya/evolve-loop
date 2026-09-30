package releasepipeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func makeHermeticGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH — skip hermetic git tests")
	}
	dir := gittest.Fixture(t).Dir
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
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-q", "-m", "feat: initial commit")
	run("tag", "v0.0.1")
	return dir
}

func TestRunChangelogGenLib_IdempotentSkip(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	clBody := "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n### Added\n- existing\n"
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(clBody), 0o644); err != nil {
		t.Fatalf("write CHANGELOG: %v", err)
	}

	err := runChangelogGenLib(dir, "v0.0.1", "HEAD", "1.0.0", false)
	if err != nil {
		t.Errorf("runChangelogGenLib idempotent skip: want nil, got %v", err)
	}

	body, _ := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if string(body) != clBody {
		t.Errorf("CHANGELOG was modified (expected idempotent skip)\ngot: %s", string(body))
	}
}

func TestRunChangelogGenLib_InvalidSemver(t *testing.T) {
	err := runChangelogGenLib(t.TempDir(), "v0.0.1", "HEAD", "not-semver", false)
	if err == nil {
		t.Fatal("runChangelogGenLib with invalid semver: want error, got nil")
	}
	if !strings.Contains(err.Error(), "not semver") {
		t.Errorf("error = %q, want mention of 'not semver'", err.Error())
	}
}

func TestRunChangelogGenLib_VerifyFromRefFails(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	err := runChangelogGenLib(dir, "nonexistent-tag", "HEAD", "2.0.0", false)
	if err == nil {
		t.Fatal("runChangelogGenLib with bad fromRef: want error, got nil")
	}
}

func TestRunChangelogGenLib_VerifyToRefFails(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	err := runChangelogGenLib(dir, "v0.0.1", "nonexistent-branch", "2.0.0", false)
	if err == nil {
		t.Fatal("runChangelogGenLib with bad toRef: want error, got nil")
	}
}

func TestRunChangelogGenLib_DryRun(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	err := runChangelogGenLib(dir, "v0.0.1", "HEAD", "2.0.0", true)
	if err != nil {
		t.Errorf("runChangelogGenLib dry-run: want nil, got %v", err)
	}

	if _, err2 := os.Stat(filepath.Join(dir, "CHANGELOG.md")); err2 == nil {
		t.Error("dry-run must NOT create CHANGELOG.md")
	}
}

func TestRunChangelogGenLib_LiveWrite(t *testing.T) {
	dir := makeHermeticGitRepo(t)

	err := runChangelogGenLib(dir, "v0.0.1", "HEAD", "2.0.0", false)
	if err != nil {
		t.Fatalf("runChangelogGenLib live write: %v", err)
	}

	body, readErr := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if readErr != nil {
		t.Fatalf("CHANGELOG.md not created: %v", readErr)
	}
	if !strings.Contains(string(body), "[2.0.0]") {
		t.Errorf("CHANGELOG.md does not contain [2.0.0] entry:\n%s", string(body))
	}
}
