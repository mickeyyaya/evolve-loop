package dossier

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo makes dir a git working tree with a commit identity configured,
// so Write(..., true) can add+commit inside it.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// gitStatus returns `git status --porcelain` output for dir (empty == clean).
func gitStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestWrite_Commit is the core of the tree-diff-guard fix: Write(d, dir, true)
// commits both files, leaving the working tree clean (no untracked pair for a
// later phase's guard to flag as a leak).
func TestWrite_Commit(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	d := &Dossier{Cycle: 99, Goal: "commit test", FinalVerdict: VerdictPass,
		Phases: []PhaseRecord{{Name: "build", Verdict: VerdictPass}}}

	if err := Write(d, dir, true); err != nil {
		t.Fatalf("Write(commit=true): %v", err)
	}
	if s := gitStatus(t, dir); s != "" {
		t.Fatalf("tree not clean after commit; git status:\n%s", s)
	}
	// Idempotent re-write of identical content must not error or make an empty commit.
	if err := Write(d, dir, true); err != nil {
		t.Fatalf("Write(commit=true) rewrite: %v", err)
	}
	if s := gitStatus(t, dir); s != "" {
		t.Fatalf("tree dirty after idempotent rewrite:\n%s", s)
	}
}

// TestWrite verifies Write creates cycle-N.json and cycle-N.md in the
// target directory (no git commit path). RED: Write doesn't exist yet.
func TestWrite(t *testing.T) {
	d := &Dossier{
		Cycle:        42,
		Goal:         "write test",
		FinalVerdict: VerdictPass,
		Phases:       []PhaseRecord{{Name: "build", Verdict: VerdictPass}},
	}
	dir := t.TempDir()
	if err := Write(d, dir, false); err != nil {
		t.Fatalf("Write: %v", err)
	}
	for _, name := range []string{"cycle-42.json", "cycle-42.md"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("Write: expected %s to exist: %v", name, err)
		}
	}
}
