//go:build acs

package cycle962

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "acs@evolve.local")
	git(t, dir, "config", "user.name", "acs")
	git(t, dir, "config", "commit.gpgsign", "false")
	writeCommit(t, dir, "base.txt", "line1\nline2\n", "base commit")
	return dir
}

func writeCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", msg)
}

func TestCarryforwardFilter_RealCherryPickRejectsConflict(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "base.txt", "CAND\nline2\n", "cand edits line1")
	git(t, dir, "checkout", "-q", "main")
	writeCommit(t, dir, "base.txt", "MAIN\nline2\n", "main edits line1")

	landable, err := core.CarryforwardCandidateLandable(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a conflicting candidate (want (false,nil)): %v", err)
	}
	if landable {
		t.Errorf("conflicting candidate reported landable=true; real 3-way cherry-pick must reject conflicts")
	}
}

func TestCarryforwardFilter_SupersededOrphanRejected(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "hello\n", "cand adds feature")
	candSHA := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))

	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "cherry-pick", candSHA)
	writeCommit(t, dir, "unrelated.txt", "x\n", "main moves on")

	landable, err := core.CarryforwardCandidateLandable(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a superseded candidate (want (false,nil)): %v", err)
	}
	if landable {
		t.Errorf("patch-id-duplicate candidate reported landable=true; supersession screen must reject already-landed work")
	}
}

func TestCarryforwardFilter_CleanNonSupersededAccepted(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "new feature\n", "cand adds feature")
	git(t, dir, "checkout", "-q", "main")
	writeCommit(t, dir, "mainonly.txt", "only on main\n", "main unrelated change")

	landable, err := core.CarryforwardCandidateLandable(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a clean candidate: %v", err)
	}
	if !landable {
		t.Errorf("clean, non-superseded candidate reported landable=false; a real feature branch must be accepted")
	}
}

func TestCarryforwardFilter_SupersededAncestorRejected(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "hello\n", "cand adds feature")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "merge", "-q", "--ff-only", "cand")

	landable, err := core.CarryforwardCandidateLandable(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on an ancestor candidate (want (false,nil)): %v", err)
	}
	if landable {
		t.Errorf("ancestor candidate reported landable=true; is-ancestor supersession screen must reject it")
	}
}

func TestPruneSupersededOrphans_FunctionalDuplicateFlagged(t *testing.T) {
	dir := initRepo(t)

	git(t, dir, "checkout", "-q", "-b", "cycle-100")
	writeCommit(t, dir, "dup.txt", "landed\n", "cycle-100 work")
	dupSHA := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))

	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "checkout", "-q", "-b", "cycle-101")
	writeCommit(t, dir, "distinct.txt", "unique\n", "cycle-101 work")

	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "cherry-pick", dupSHA)

	noOpenPR := func(string) (bool, error) { return false, nil }
	verdicts, err := core.PruneSupersededOrphans(context.Background(), dir, "main", noOpenPR)
	if err != nil {
		t.Fatalf("PruneSupersededOrphans returned error: %v", err)
	}

	dup := findVerdict(t, verdicts, "cycle-100")
	if !dup.Superseded {
		t.Errorf("cycle-100 (already landed on main) not flagged Superseded")
	}
	if !dup.Pruned {
		t.Errorf("cycle-100 superseded with no open PR should be Pruned=true")
	}
	distinct := findVerdict(t, verdicts, "cycle-101")
	if distinct.Superseded {
		t.Errorf("cycle-101 (distinct work) wrongly flagged Superseded; different-goal orphans must be left alone")
	}
	if distinct.Pruned {
		t.Errorf("cycle-101 (distinct work) wrongly Pruned; different-goal orphans must not be deleted")
	}
}

func TestPruneSupersededOrphans_OpenPRNotDeleted(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cycle-200")
	writeCommit(t, dir, "dup.txt", "landed\n", "cycle-200 work")
	dupSHA := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "cherry-pick", dupSHA)

	hasOpenPR := func(ref string) (bool, error) { return true, nil }
	verdicts, err := core.PruneSupersededOrphans(context.Background(), dir, "main", hasOpenPR)
	if err != nil {
		t.Fatalf("PruneSupersededOrphans returned error: %v", err)
	}

	v := findVerdict(t, verdicts, "cycle-200")
	if !v.Superseded {
		t.Errorf("cycle-200 (already landed) not flagged Superseded")
	}
	if v.Pruned {
		t.Errorf("cycle-200 has an open PR; must be flagged but NOT pruned (verify_remote_pr_before_branch_delete)")
	}
	if out := strings.TrimSpace(git(t, dir, "branch", "--list", "cycle-200")); out == "" {
		t.Errorf("cycle-200 branch was deleted despite an open PR; prune must be skipped")
	}
}

func findVerdict(t *testing.T, verdicts []core.OrphanVerdict, refSuffix string) core.OrphanVerdict {
	t.Helper()
	for _, v := range verdicts {
		if strings.Contains(v.Ref, refSuffix) {
			return v
		}
	}
	t.Fatalf("no OrphanVerdict for %q in %+v", refSuffix, verdicts)
	return core.OrphanVerdict{}
}
