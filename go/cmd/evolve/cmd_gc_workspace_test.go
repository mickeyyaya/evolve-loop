package main

// The fixture helpers (gcGit / gcWorktreeEnv / gcOrphanBranch) are the ones
// already used by cmd_loop_gc_worktree_test.go in this package.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gcUnmergedBranch is a cycle-* branch carrying a commit that is NOT reachable
// from HEAD — unlanded cycle work the sweep must refuse to delete.
const gcUnmergedBranch = "cycle-888"

func gcAddUnmergedBranch(t *testing.T, projectRoot string) {
	t.Helper()
	gcGit(t, projectRoot, "checkout", "-b", gcUnmergedBranch)
	if err := os.WriteFile(filepath.Join(projectRoot, "unlanded.txt"), []byte("unlanded work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gcGit(t, projectRoot, "add", "unlanded.txt")
	gcGit(t, projectRoot, "commit", "-m", "unlanded cycle work")
	gcGit(t, projectRoot, "checkout", "main")
}

func TestRunGC_DryRunPrintsWorkspacePlanAndMutatesNothing(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "") // repo + merged orphan branch cycle-777

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)

	if rc == 10 {
		t.Fatalf("runGC rejected its arguments (rc=10) — `evolve gc --project-root` is not implemented; stderr=%s", stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, gcOrphanBranch) {
		t.Errorf("`evolve gc --dry-run` output does not name the merged orphan branch %s — the worktree/branch sweep has no operator surface:\n%s", gcOrphanBranch, out)
	}
	if !strings.Contains(out, "WOULD-") {
		t.Errorf("`evolve gc --dry-run` output has no WOULD- planned-action lines for the workspace sweep:\n%s", out)
	}
	if !gcBranchExists(t, projectRoot, gcOrphanBranch) {
		t.Errorf("--dry-run DELETED branch %s — a preview must mutate nothing", gcOrphanBranch)
	}
}

func TestRunGC_ExplicitRunAppliesWorkspaceSweep(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")

	var stdout, stderr bytes.Buffer
	rc := runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)

	if rc == 10 {
		t.Fatalf("runGC rejected its arguments (rc=10); stderr=%s", stderr.String())
	}
	if gcBranchExists(t, projectRoot, gcOrphanBranch) {
		t.Errorf("an explicit `evolve gc` left merged orphan branch %s in place — the operator run must APPLY the workspace sweep (enforce), not preview it; stdout=%s stderr=%s", gcOrphanBranch, stdout.String(), stderr.String())
	}
}

func TestRunGC_ExplicitRunPreservesUnmergedBranch(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcAddUnmergedBranch(t, projectRoot)

	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr); rc == 10 {
		t.Fatalf("runGC rejected its arguments (rc=10); stderr=%s", stderr.String())
	}

	if !gcBranchExists(t, projectRoot, gcUnmergedBranch) {
		t.Errorf("`evolve gc` deleted UNMERGED branch %s — unlanded cycle work must never be reaped; stdout=%s stderr=%s", gcUnmergedBranch, stdout.String(), stderr.String())
	}
}

func TestRunGC_MutatingRunRefusesWithoutProjectRoot(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runGC([]string{}, nil, &stdout, &stderr)

	if rc != 1 {
		t.Fatalf("expected runGC to fail (rc=1) when --project-root is omitted in a mutating run, got %d", rc)
	}
	if !strings.Contains(stderr.String(), "mutating run refused: --project-root must be explicitly set") {
		t.Errorf("expected stderr to contain the refusal message, got: %s", stderr.String())
	}
}
