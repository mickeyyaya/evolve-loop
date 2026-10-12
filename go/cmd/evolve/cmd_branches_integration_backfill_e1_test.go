//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func branchesRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	r.Git("branch", "cycle-100")
	r.Git("commit", "-q", "--allow-empty", "-m", "advance main")
	return r
}

func runBranchesIn(dir string, args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := runBranches(append(args, "--project-root", dir), nil, &out, &errb)
	return out.String(), errb.String(), code
}

func TestBranches_ABadFlagIsAUsageError(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"audit", "prune"} {
		out, errOut, code := runBranchesIn(t.TempDir(), verb, "--no-such-flag")
		if code != 10 || out != "" || !strings.Contains(errOut, "flag provided but not defined: -no-such-flag") {
			t.Errorf("%s: code=%d stdout=%q stderr=%q, want 10 and the flag error", verb, code, out, errOut)
		}
	}
}

func TestBranches_ANonRepositoryFailsWithTheListingError(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"audit", "prune"} {
		out, errOut, code := runBranchesIn(t.TempDir(), verb)
		if code != 1 || out != "" || !strings.HasPrefix(errOut, "evolve branches "+verb+": prune orphans: for-each-ref exited 128") {
			t.Errorf("%s: code=%d stdout=%q stderr=%q, want 1 and the listing error", verb, code, out, errOut)
		}
	}
}

func TestBranchesAudit_AnUnrelatedHistoryRefFailsTheLandableCheck(t *testing.T) {
	t.Parallel()
	r := branchesRepo(t)
	r.Git("checkout", "-q", "--orphan", "cycle-300")
	if err := os.WriteFile(filepath.Join(r.Dir, "x.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "x.txt")
	r.Git("commit", "-q", "-m", "orphan")
	r.Git("checkout", "-q", "main")

	out, errOut, code := runBranchesIn(r.Dir, "audit")

	if code != 1 {
		t.Fatalf("code = %d, want 1 (stdout=%q stderr=%q)", code, out, errOut)
	}
	if !strings.Contains(errOut, "evolve branches audit: landable(cycle-300): carryforward filter: merge-tree main cycle-300 exited 128") {
		t.Errorf("stderr = %q, want the landable error naming the ref", errOut)
	}
	if !strings.Contains(out, "cycle-100 superseded=true landable=false\n") {
		t.Errorf("stdout = %q, want the refs before the failing one still reported", out)
	}
}

func TestBranchesPrune_AnOpenRemotePRKeepsASupersededRef(t *testing.T) {
	r := branchesRepo(t)
	r.Git("remote", "add", "origin", filepath.Join(t.TempDir(), "absent.git"))
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\nfor a in \"$@\"; do\n  if [ \"$a\" = \"-C\" ]; then\n    echo \"unknown shorthand flag: 'C' in -C\" >&2\n    exit 1\n  fi\ndone\necho 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, errOut, code := runBranchesIn(r.Dir, "prune", "--dry-run=false")

	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errOut)
	}
	if out != "cycle-100 superseded=true kept-open-pr\n" {
		t.Errorf("stdout = %q, want the superseded ref kept for its open PR", out)
	}
	if r.Git("branch", "--list", "cycle-100") == "" {
		t.Error("cycle-100 was deleted despite its open PR")
	}
}
