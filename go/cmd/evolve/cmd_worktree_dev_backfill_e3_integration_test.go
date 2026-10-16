//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestRunWorktreeCreateDev_AShowRefFaultIsAnIOErrorNotAFreeBranch(t *testing.T) {
	runtime, _ := devHubFixture(t)
	gittest.InstallFault(t, gittest.GitFault{Verb: "show-ref", Code: 128, Stderr: "fatal: bad object"})

	out, code := e3DevCreate(t, runtime, "t1", "b1")

	if code != 2 || !strings.Contains(out, "evolve worktree create --dev: git show-ref rc=128 err=<nil>: fatal: bad object") {
		t.Fatalf("code %d out %q", code, out)
	}
	store := filepath.Join(filepath.Dir(runtime), ".repo.git")
	if strings.Contains(out, "already exists") || gittest.BranchExists(t, store, "b1") {
		t.Errorf("a show-ref fault was read as an answer: %q", out)
	}
}
