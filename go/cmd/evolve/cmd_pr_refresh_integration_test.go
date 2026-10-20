//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/researchindex"
)

func TestPRRefresh_TheRealGeneratorsOfACloneCommitOnlyTheStaleIndex(t *testing.T) {
	t.Setenv("GOFLAGS", "-mod=readonly")
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	clone := gittest.Clone(t, root)
	if code, stdout, stderr := runVerb(prVerb, "refresh", "--project-root", clone.Dir); code != 0 || !strings.Contains(stdout, "up to date") {
		t.Fatalf("precondition: the derived files of HEAD must be in sync before the test adds an entry: %d %q %q", code, stdout, stderr)
	}
	clone.Write(researchindex.EntriesDir+"/2099-01-01-01-e2e.md", "## 2099-01-01 — the end-to-end entry\n")
	clone.Git("add", "-A")
	clone.Git("commit", "-q", "-m", "docs: an entry with no index line")

	code, stdout, stderr := runVerb(prVerb, "refresh", "--project-root", clone.Dir)

	if code != 0 || !strings.Contains(stdout, "committed") {
		t.Fatalf("refresh = %d %q %q", code, stdout, stderr)
	}
	if subject := clone.Git("log", "-1", "--format=%s"); subject != prRefreshMessage {
		t.Errorf("the refresh commit is %q, want %q", subject, prRefreshMessage)
	}
	if files := clone.Git("show", "--name-only", "--format=", "HEAD"); files != researchindex.IndexPath {
		t.Errorf("the refresh commit holds %q, want only %s: the signal codes and the darwin set of a clean tree are in sync", files, researchindex.IndexPath)
	}
	index, err := os.ReadFile(filepath.Join(clone.Dir, researchindex.IndexPath))
	if err != nil || !strings.Contains(string(index), "## 2099-01-01 — the end-to-end entry") {
		t.Errorf("the index lacks the new entry (err %v)", err)
	}
}
