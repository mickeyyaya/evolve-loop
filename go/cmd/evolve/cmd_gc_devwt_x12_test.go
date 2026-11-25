package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func x12LandedTree(h gcDevHubEnv, name string) string {
	h.t.Helper()
	dir := h.addDevAt(name, 0)
	h.store.Git("update-ref", "refs/remotes/origin/main", h.store.Git("-C", dir, "rev-parse", "HEAD"))
	return dir
}

func x12ClosedPRs(t *testing.T, branch, oid string) {
	t.Helper()
	rows := `[{"headRefName":"` + branch + `","headRefOid":"` + oid + `","state":"CLOSED","updatedAt":"2026-10-01T00:00:00Z"}]`
	fakeGhOnPath(t, `case "$*" in *"--state open"*) echo '[]';; *) echo '`+rows+`';; esac`)
}

func TestRunGC_X12_AClosedUnmergedCleanTreeIsRemovedAndItsBranchStays(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	closed := h.addDevAt("closed-lane", 0)
	keep := h.addDevAt("unlanded-lane", 0)
	x12ClosedPRs(t, "br-closed-lane", h.store.Git("-C", closed, "rev-parse", "HEAD"))

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--project-root", h.runtime}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(closed); !os.IsNotExist(err) {
		t.Errorf("the clean tree of a closed pull request stayed (stat err %v)\n%s", err, stdout.String())
	}
	if strings.TrimSpace(h.store.Git("branch", "--list", "br-closed-lane")) == "" {
		t.Error("the branch of the removed tree was deleted")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("a tree with no pull request was removed: %v", err)
	}
}

func TestRunGC_X12_ADirtyLandedTreeIsKeptAndItsChangedPathsAreListed(t *testing.T) {
	h := newGCDevHub(t, `{}`)
	fakeGhOnPath(t, `echo '[]'`)
	dirty := x12LandedTree(h, "dirty-lane")
	if err := os.WriteFile(filepath.Join(dirty, "base.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirty, "scratch.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--dry-run", "--project-root", h.runtime}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d; stderr=%s", rc, stderr.String())
	}
	var line string
	for _, l := range strings.Split(stdout.String(), "\n") {
		if strings.Contains(l, "path=") && strings.Contains(l, "dirty-lane") {
			line = l
		}
	}
	if !strings.Contains(line, "KEEP") {
		t.Fatalf("no KEEP line for the dirty tree:\n%s", stdout.String())
	}
	for _, rel := range []string{"base.txt", "scratch.txt"} {
		if !strings.Contains(line, rel) {
			t.Errorf("the line %q does not list the changed path %s", line, rel)
		}
	}
}

func TestRunGC_X12_APolicyProtectedGlobKeepsALandedTreeAndThePolicyLoadsClean(t *testing.T) {
	h := newGCDevHub(t, `{"gc":{"dev_worktrees":{"protected":["code-*"]}}}`)
	fakeGhOnPath(t, `echo '[]'`)
	protected := x12LandedTree(h, "code-index")

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--project-root", h.runtime}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "policy load failed") {
		t.Errorf("policy.json with gc.dev_worktrees.protected does not load:\n%s", stderr.String())
	}
	if _, err := os.Stat(protected); err != nil {
		t.Errorf("the tree that the glob protects was removed: %v", err)
	}
	if !strings.Contains(stdout.String(), "protected by policy: code-*") {
		t.Errorf("the output does not name the glob:\n%s", stdout.String())
	}
}
