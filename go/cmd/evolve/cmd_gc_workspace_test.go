package main

// cmd_gc_workspace_test.go — RED tests for cycle-1172, inbox item
// `workspace-hygiene-s5-wiring-shadow-default` (scout task
// evolve-gc-workspace-sweep-dry-run).
//
// THE GAP: `evolve gc` (cmd_gc.go) reaps only orphan tmux sessions and dead
// per-run sockets. The S4/S5 worktree+branch sweep (gc.PlanWorktrees /
// gc.ApplyWorktrees) has NO operator surface at all: today it is observable
// only as a JSON manifest written mid-batch by runGCHook. An operator who
// wants to know what the sweep would do — or to drain the backlog on demand
// after a crash — has no command to run.
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//
//  1. `evolve gc` gains a `--project-root` parameter (defaulting to the process
//     cwd) so the sweep is aimed explicitly. Never infer the repo implicitly
//     for a MUTATING run — same refusal posture runWorktreeGC already takes on
//     an empty ProjectRoot.
//  2. `evolve gc --dry-run` additionally plans the worktree/branch sweep via
//     gc.PlanWorktrees and prints every planned item, each line naming the
//     branch and prefixed `WOULD-` (parity with the existing orphan-session
//     dry-run vocabulary). It mutates NOTHING.
//  3. A bare `evolve gc` (no --dry-run) APPLIES the worktree sweep. This is the
//     documented asymmetry: an explicit operator run is enforce, while the
//     in-loop hook's default stays shadow (policy.json owns the loop's mode;
//     the operator owns their own invocation). Document it in the command help.
//  4. NEGATIVE — the apply must never touch UNMERGED work: gc.PlanWorktrees
//     flags an unmerged cycle-* branch instead of planning a delete, and the
//     CLI must not upgrade that to a deletion. Unlanded cycle work is exactly
//     what this backlog is protecting.
//
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

// gcAddUnmergedBranch creates gcUnmergedBranch with one commit of its own and
// returns the repo to main.
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

// TestRunGC_DryRunPrintsWorkspacePlanAndMutatesNothing — AC2.1/AC2.2. The
// dry-run must SHOW the worktree/branch plan (naming the merged orphan branch,
// not just a count) and leave the repo byte-identical. RED today: `evolve gc`
// has no worktree sweep and rejects --project-root outright.
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

// TestRunGC_ExplicitRunAppliesWorkspaceSweep — AC2.3, the documented
// asymmetry: an explicit operator `evolve gc` is enforce, so the merged orphan
// branch is really gone afterwards. This is the anti-no-op half — an
// implementation that only ever previews passes the test above and fails here.
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

// TestRunGC_ExplicitRunPreservesUnmergedBranch — AC2.4, NEGATIVE. Enforce on an
// operator run must still refuse unlanded work: an unmerged cycle-* branch is
// flagged, never deleted (gc.ApplyWorktrees uses `git branch -d`, never -D).
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

// TestRunGC_MutatingRunRefusesWithoutProjectRoot — AC2.1, explicitly aim the sweep.
// A bare `evolve gc` without --project-root must fail rather than guessing cwd,
// avoiding accidental destructive side effects.
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
