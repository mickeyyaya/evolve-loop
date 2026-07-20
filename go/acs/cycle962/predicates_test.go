//go:build acs

// Package cycle962 materializes the cycle-962 acceptance criteria for this
// fleet lane's committed work under inbox item scout-carryforward-real-
// cherrypick-filter (weight 0.94, campaign merge-efficiency-2026-07). Triage
// committed TWO coherent top_n tasks to this single lane/worktree:
//
//	carryforward-real-cherrypick-filter (PRIMARY) — a deterministic, zero-LLM
//	    Go filter CarryforwardCandidateLandable that replaces the bare
//	    `git merge-tree` conflict oracle (reports clean on real conflicts, no
//	    functional-duplicate screen) with a REAL 3-way cherry-pick dry-run plus
//	    an is-ancestor / patch-id supersession screen.
//	prune-superseded-orphans-lane (dependent) — a housekeeping walker
//	    PruneSupersededOrphans that flags/prunes stale orphan `cycle-*` refs
//	    using the supersession screen, honoring verify_remote_pr_before_branch_delete.
//
// SCOPE NOTE (Rule 3, surfaced): fleet_scope names a single inbox id; triage
// expanded it into these two committed tasks in ONE worktree (Task 2 dependsOn
// Task 1). Predicates are authored for BOTH because both are triage top_n (not
// deferred) and built together here. The two DEFERRED beyond-ask ideas
// (rung0-dispatch-merge-tree-precheck, generic-already-landed-utility) get
// ZERO predicates (R9.3 floor-binding).
//
// SUT surface the Builder must add to package core
// (go/internal/core/carryforward_filter.go), WITHOUT modifying this file:
//
//	func CarryforwardCandidateLandable(ctx context.Context, dir, candidateRef, base string) (bool, error)
//	    // true  → candidate cleanly 3-way cherry-picks onto base AND is not
//	    //         already landed (not is-ancestor of base, not patch-id dup).
//	    // false → any real cherry-pick conflict, OR superseded (is-ancestor /
//	    //         patch-id dup of base). Zero-LLM. Git via the gitCapture seam.
//	    // err   → git infrastructure failure only (never for a conflict).
//
//	type OrphanVerdict struct { Ref string; Superseded bool; Pruned bool }
//	func PruneSupersededOrphans(ctx context.Context, dir, base string, hasOpenPR func(ref string) (bool, error)) ([]OrphanVerdict, error)
//	    // Walks local `cycle-*` branches. Superseded=true when the branch is a
//	    // functional duplicate already on base (same screen as Task 1).
//	    // Pruned=true ONLY when Superseded && hasOpenPR(ref)==false (delete the
//	    // stale ref); an open PR / remote leaves it flagged-but-kept.
//	    // Different-goal (non-superseded) orphans are left untouched.
//
// PREDICATE STYLE (cycle-85 rule): go/internal/core is importable from go/acs,
// so every predicate EXERCISES the SUT directly against a REAL git repo built
// in a temp dir (git is always present) and asserts on the returned value —
// no source-grep predicate exists in this file. RED here is a COMPILE failure
// (undefined: core.CarryforwardCandidateLandable / core.PruneSupersededOrphans),
// which fails for the right reason: the production symbols are absent.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C962_003 (clean, non-superseded candidate is ACCEPTED — the
//	           anti-`return false` signal a no-op filter cannot fake).
//	NEGATIVE → C962_001 (genuine cherry-pick conflict REJECTED — the strongest
//	           anti-no-op: a filter that trusts bare merge-tree accepts this),
//	           C962_002 (patch-id-dup already-landed REJECTED),
//	           C962_004 (is-ancestor already-landed REJECTED).
//	EDGE     → C962_005 pins different-goal-left-alone vs same-goal-flagged in
//	           ONE walk; C962_006 pins the open-PR guard (flagged, NOT pruned).
//	SEMANTIC → accept/reject and flag/prune are DISTINCT outcomes, each asserted
//	           separately (not one behavior restated).
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 merge-tree-clean-but-real-conflict → rejected      → C962_001 (NEGATIVE)
//	AC2 patch-id-dup already-landed        → rejected      → C962_002 (NEGATIVE)
//	AC3 clean, non-superseded              → accepted      → C962_003 (POSITIVE)
//	AC4 is-ancestor already-landed         → rejected      → C962_004 (NEGATIVE/EDGE)
//	AC5 same-goal dup flagged, other-goal left alone       → C962_005 (EDGE/SEMANTIC)
//	AC6 superseded + open PR → flagged, NOT deleted        → C962_006 (NEGATIVE/EDGE)
//	AC7 -race clean / go vet clean / apicover clean        → manual+checklist (Auditor CI-parity)
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

// git runs `git -C dir args...` as the TEST HARNESS (not the SUT) and fails the
// test on any non-zero exit — fixtures must build cleanly for the assertion on
// the SUT to mean anything.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}

// initRepo creates a fresh git repo on branch `main` with one committed file
// base.txt (two lines) and deterministic identity/config, returning its path.
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

// writeCommit writes content to name under dir and commits it with msg.
func writeCommit(t *testing.T, dir, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-q", "-m", msg)
}

// -----------------------------------------------------------------------------
// AC1 (NEGATIVE) — a candidate that modifies the SAME line main later changed a
// different way produces a REAL 3-way cherry-pick conflict. A filter trusting a
// bare 1-arg `git merge-tree` (which is not a 3-way merge and reports no index)
// would accept this; CarryforwardCandidateLandable MUST reject it.
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

// AC2 (NEGATIVE) — candidate's change is already on main under a DIFFERENT sha
// (cherry-picked), with an extra unrelated main commit so the candidate tip is
// NOT an ancestor. Only a patch-id functional-duplicate screen catches this.
func TestCarryforwardFilter_SupersededOrphanRejected(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "hello\n", "cand adds feature")
	candSHA := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))

	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "cherry-pick", candSHA) // same change, new sha → patch-id dup
	writeCommit(t, dir, "unrelated.txt", "x\n", "main moves on")

	landable, err := core.CarryforwardCandidateLandable(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on a superseded candidate (want (false,nil)): %v", err)
	}
	if landable {
		t.Errorf("patch-id-duplicate candidate reported landable=true; supersession screen must reject already-landed work")
	}
}

// AC3 (POSITIVE, anti-`return false`) — a candidate adding a NEW file, with main
// diverged only by an unrelated file, cleanly cherry-picks and is not
// superseded. MUST be accepted; a filter that always rejects fails here.
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

// AC4 (NEGATIVE / EDGE) — candidate tip is a strict ancestor of main (already
// fast-forward merged). The is-ancestor arm of the supersession screen must
// reject it even though a cherry-pick of an ancestor is an empty no-op.
func TestCarryforwardFilter_SupersededAncestorRejected(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "cand")
	writeCommit(t, dir, "feature.txt", "hello\n", "cand adds feature")
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "merge", "-q", "--ff-only", "cand") // main tip == cand tip

	landable, err := core.CarryforwardCandidateLandable(context.Background(), dir, "cand", "main")
	if err != nil {
		t.Fatalf("unexpected error on an ancestor candidate (want (false,nil)): %v", err)
	}
	if landable {
		t.Errorf("ancestor candidate reported landable=true; is-ancestor supersession screen must reject it")
	}
}

// -----------------------------------------------------------------------------
// AC5 (EDGE / SEMANTIC) — walk a repo with two orphan `cycle-*` branches: one is
// a functional duplicate already landed on main (flagged + pruned), the other is
// distinct work (left alone). Asserts the walker DISTINGUISHES them in one pass.
func TestPruneSupersededOrphans_FunctionalDuplicateFlagged(t *testing.T) {
	dir := initRepo(t)

	// cycle-100: change already landed on main (patch-id dup) → superseded.
	git(t, dir, "checkout", "-q", "-b", "cycle-100")
	writeCommit(t, dir, "dup.txt", "landed\n", "cycle-100 work")
	dupSHA := strings.TrimSpace(git(t, dir, "rev-parse", "HEAD"))

	// cycle-101: distinct work NOT on main → not superseded.
	git(t, dir, "checkout", "-q", "main")
	git(t, dir, "checkout", "-q", "-b", "cycle-101")
	writeCommit(t, dir, "distinct.txt", "unique\n", "cycle-101 work")

	// main absorbs cycle-100's change under a new sha.
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

// AC6 (NEGATIVE / EDGE) — a superseded orphan with an OPEN PR must be flagged but
// NOT pruned (verify_remote_pr_before_branch_delete). The injected hasOpenPR
// returns true; Superseded=true yet Pruned=false and the ref survives.
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
	// Ref must still exist — prune must not have run.
	if out := strings.TrimSpace(git(t, dir, "branch", "--list", "cycle-200")); out == "" {
		t.Errorf("cycle-200 branch was deleted despite an open PR; prune must be skipped")
	}
}

// findVerdict returns the verdict whose Ref contains refSuffix (Ref may be a
// short name or a fully-qualified refs/heads/ path), failing if absent.
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
