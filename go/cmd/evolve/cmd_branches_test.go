package main

// cmd_branches_test.go — TDD red-first unit contract for the `evolve branches`
// subcommand (cycle-969, wire-carryforward-prune-cli). These tests call
// runBranches directly against a real temp git repo (no remote), so they run in
// the normal suite (`go test ./cmd/evolve/...`) with no `acs` build tag and no
// binary build — the fast red/green loop the Builder codes against. The
// worktree-gating, durable predicates live in go/acs/cycle969/predicates_test.go.
//
// RED before the Builder acts: runBranches is undefined, so this file fails to
// COMPILE — the whole package test reds for the right reason (the SUT is absent).
//
// The Builder must NOT modify this file; it adds go/cmd/evolve/cmd_branches.go
// (runBranches) and the registry.go row. Contract mirrored from the ACS package:
//   - audit  → read-only; per branch prints `superseded=<t|f> landable=<t|f>`
//              (dispatching to core.PruneSupersededOrphans AND
//              core.CarryforwardCandidateLandable).
//   - prune  → default dry-run (deletes nothing, flags `would-prune`);
//              --dry-run=false deletes each superseded ref whose hasOpenPR is
//              false. With no remote configured hasOpenPR MUST degrade to
//              (false, nil) (verify_remote_pr_before_branch_delete).

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func brGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func brCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	brGit(t, dir, "add", name)
	brGit(t, dir, "commit", "-q", "-m", msg)
}

// brFixture builds a remote-less repo on `main` with a superseded (ancestor)
// cycle-100 and a divergent-clean cycle-200 — same shape as the ACS fixture.
func brFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	brGit(t, dir, "init", "-q", "-b", "main")
	brGit(t, dir, "config", "user.email", "acs@evolve.local")
	brGit(t, dir, "config", "user.name", "acs")
	brGit(t, dir, "config", "commit.gpgsign", "false")
	brCommit(t, dir, "base.txt", "line1\n", "base commit")
	brGit(t, dir, "branch", "cycle-100")
	brGit(t, dir, "checkout", "-q", "-b", "cycle-200")
	brCommit(t, dir, "feature.txt", "new work\n", "cycle-200 unique commit")
	brGit(t, dir, "checkout", "-q", "main")
	brCommit(t, dir, "base.txt", "line1\nline2\n", "advance main")
	return dir
}

func brBranchExists(t *testing.T, dir, name string) bool {
	t.Helper()
	return exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run() == nil
}

func brRun(t *testing.T, dir string, args ...string) (stdout string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	code = runBranches(append(args, "--project-root", dir, "--base", "main"), nil, &out, &errb)
	if errb.Len() > 0 {
		t.Logf("stderr: %s", errb.String())
	}
	return out.String(), code
}

func brLine(stdout, ref string, needles ...string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		if !strings.Contains(line, ref) {
			continue
		}
		ok := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// TestBranchesAudit_ReportsSupersededAndLandable exercises BOTH core functions
// through the audit dispatcher and confirms audit is read-only.
func TestBranchesAudit_ReportsSupersededAndLandable(t *testing.T) {
	dir := brFixture(t)
	stdout, code := brRun(t, dir, "audit")
	if code != 0 {
		t.Fatalf("audit exit=%d (want 0)\n%s", code, stdout)
	}
	if !brLine(stdout, "cycle-100", "superseded=true", "landable=false") {
		t.Errorf("cycle-100 must report superseded=true landable=false\n%s", stdout)
	}
	if !brLine(stdout, "cycle-200", "superseded=false", "landable=true") {
		t.Errorf("cycle-200 must report superseded=false landable=true\n%s", stdout)
	}
	if !brBranchExists(t, dir, "cycle-100") {
		t.Errorf("audit must be read-only: cycle-100 was deleted")
	}
}

// TestBranchesPruneDryRunDefault_KeepsSuperseded: default prune deletes nothing.
func TestBranchesPruneDryRunDefault_KeepsSuperseded(t *testing.T) {
	dir := brFixture(t)
	stdout, code := brRun(t, dir, "prune")
	if code != 0 {
		t.Fatalf("prune (default) exit=%d (want 0)\n%s", code, stdout)
	}
	if !brBranchExists(t, dir, "cycle-100") {
		t.Errorf("default prune must be dry-run: cycle-100 was deleted")
	}
	if !brLine(stdout, "cycle-100", "would-prune") {
		t.Errorf("default prune must flag cycle-100 as would-prune\n%s", stdout)
	}
}

// TestBranchesPruneForce_DeletesSupersededKeepsDivergent: --dry-run=false prunes
// the superseded ref (no remote → hasOpenPR false) and leaves the divergent one.
func TestBranchesPruneForce_DeletesSupersededKeepsDivergent(t *testing.T) {
	dir := brFixture(t)
	stdout, code := brRun(t, dir, "prune", "--dry-run=false")
	if code != 0 {
		t.Fatalf("prune --dry-run=false exit=%d (want 0)\n%s", code, stdout)
	}
	if brBranchExists(t, dir, "cycle-100") {
		t.Errorf("prune --dry-run=false must delete superseded cycle-100\n%s", stdout)
	}
	if !brBranchExists(t, dir, "cycle-200") {
		t.Errorf("prune must NOT delete divergent cycle-200")
	}
}

// TestBranchesUnknownSubcommand_Errors: an unknown subcommand is a non-zero exit.
func TestBranchesUnknownSubcommand_Errors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := runBranches([]string{"bogus"}, nil, &out, &errb); code == 0 {
		t.Errorf("unknown subcommand must exit non-zero, got 0")
	}
}
