//go:build integration

package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWorktreeCreateDev_AShowRefFaultIsAnIOErrorNotAFreeBranch(t *testing.T) {
	runtime, _ := devHubFixture(t)
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	bfStub(t, "git", `for a in "$@"; do
  if [ "$a" = show-ref ]; then echo "fatal: bad object" >&2; exit 128; fi
done
exec '`+realGit+`' "$@"`)

	out, code := e3DevCreate(t, runtime, "t1", "b1")

	if code != 2 || !strings.Contains(out, "evolve worktree create --dev: git show-ref rc=128 err=<nil>: fatal: bad object") {
		t.Fatalf("code %d out %q", code, out)
	}
	store := filepath.Join(filepath.Dir(runtime), ".repo.git")
	if strings.Contains(out, "already exists") || exec.Command(realGit, "-C", store, "show-ref", "--verify", "--quiet", "refs/heads/b1").Run() == nil {
		t.Errorf("a show-ref fault was read as an answer: %q", out)
	}
}
