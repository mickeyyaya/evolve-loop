package dossier

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	if err := Write(d, dir, true); err != nil {
		t.Fatalf("Write(commit=true) rewrite: %v", err)
	}
	if s := gitStatus(t, dir); s != "" {
		t.Fatalf("tree dirty after idempotent rewrite:\n%s", s)
	}
}

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
