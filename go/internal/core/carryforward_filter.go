// carryforward_filter.go — deterministic, zero-LLM carry-forward candidate
// screen (cycle 962, inbox item scout-carryforward-real-cherrypick-filter,
// weight 0.94, campaign merge-efficiency-2026-07).
//
// The fleet-rebase carry-forward candidate selection was driven by the
// LLM scout/triage phase running raw git with a bare 1-arg `git merge-tree`
// as its conflict oracle. That legacy form is NOT a 3-way merge, produces no
// index, and reports "clean" on real conflicts — and it has no
// functional-duplicate screen — so already-superseded orphan `cycle-*`
// branches were mis-selected and burned operator-prioritized rebase cycles
// (cycle-826 Opus auditor reproduction: 6 conflict indicators / 36 markers on
// a merge-tree-"clean" candidate).
//
// The root-cause fix is a deterministic Go filter the phase can call instead
// of eyeballing raw git: a REAL 3-way merge dry-run (`git merge-tree
// --write-tree`, which writes only to the object DB — HEAD/index/worktree are
// untouched) plus an is-ancestor / patch-id supersession screen. All git runs
// through the gitCapture seam so it is fakeable in fast-tier tests.
package core

import (
	"context"
	"fmt"
	"strings"
)

// CarryforwardCandidateLandable reports whether candidateRef is worth carrying
// forward onto base. It is deterministic and LLM-free.
//
//	true  → candidate cleanly 3-way merges onto base AND is not already landed.
//	false → any real merge conflict, OR superseded (candidateRef is an ancestor
//	        of base, or every candidate commit is already on base by patch-id).
//	err   → git-infrastructure failure only; a genuine conflict returns
//	        (false, nil), never an error.
//
// Order matters: the supersession screen runs first so an already-landed
// candidate is rejected before the merge dry-run (whose merge of an
// already-absorbed change would read as "clean" and mask the duplicate).
func CarryforwardCandidateLandable(ctx context.Context, dir, candidateRef, base string) (bool, error) {
	superseded, err := refSuperseded(ctx, dir, candidateRef, base)
	if err != nil {
		return false, err
	}
	if superseded {
		return false, nil
	}

	// Real 3-way merge dry-run. `merge-tree --write-tree` uses the merge-base
	// of the two commits and reports conflicts via the exit code (0=clean,
	// 1=conflict) without touching HEAD/index/worktree.
	_, code, err := gitCapture(ctx, dir, "merge-tree", "--write-tree", base, candidateRef)
	if err != nil {
		return false, fmt.Errorf("carryforward filter: merge-tree %s %s: %w", base, candidateRef, err)
	}
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil // genuine 3-way conflict — not landable, not an error
	default:
		return false, fmt.Errorf("carryforward filter: merge-tree %s %s exited %d", base, candidateRef, code)
	}
}

// FleetRebaseVerdict is the deterministic three-way classification of a
// fleet-rebase carry-forward candidate, produced by ClassifyFleetRebaseCandidate
// as a pre-screen before the blind rebase replay in the ship-recovery path.
type FleetRebaseVerdict int

const (
	// FleetRebaseAlreadyLanded — the candidate is superseded (an ancestor of base,
	// or every commit already on base by patch-id). Short-circuit: no replay, no
	// re-audit (the explicit fix for the 948 "PASS-but-unlanded duplicate" waste).
	FleetRebaseAlreadyLanded FleetRebaseVerdict = iota
	// FleetRebaseClean — the candidate cleanly 3-way merges onto base and is not
	// already landed. Worth replaying: proceed to the existing rebase path.
	FleetRebaseClean
	// FleetRebaseConflict — a genuine 3-way conflict (real overlapping work). Must
	// route to the debugger, NEVER be short-circuited as AlreadyLanded.
	FleetRebaseConflict
)

// ClassifyFleetRebaseCandidate is the deterministic, zero-LLM pre-screen for the
// fleet-rebase recovery path. It REUSES CarryforwardCandidateLandable (giving that
// otherwise-inert cycle-962 surface its first production caller) and splits its
// false result into the two outcomes that must be handled differently:
//
//	FleetRebaseClean         → landable (clean 3-way merge, not superseded)
//	FleetRebaseAlreadyLanded → not landable BECAUSE superseded (short-circuit)
//	FleetRebaseConflict      → not landable BECAUSE a genuine conflict (→ debugger)
//	err                      → git-infrastructure failure only; never masked as a verdict.
//
// The naive "landable==false ⇒ already landed" reading is WRONG: landable is false
// for BOTH a superseded candidate AND a real conflict, so a second supersession
// screen disambiguates them — mislabelling a conflict as landed would silently drop
// real overlapping work.
func ClassifyFleetRebaseCandidate(ctx context.Context, dir, candidateRef, base string) (FleetRebaseVerdict, error) {
	landable, err := CarryforwardCandidateLandable(ctx, dir, candidateRef, base)
	if err != nil {
		return FleetRebaseAlreadyLanded, err
	}
	if landable {
		return FleetRebaseClean, nil
	}
	// Not landable: superseded (already landed) or a genuine conflict. Re-run the
	// supersession screen to disambiguate — the same deterministic git checks the
	// filter already used, so the classification stays coherent with it.
	superseded, err := refSuperseded(ctx, dir, candidateRef, base)
	if err != nil {
		return FleetRebaseAlreadyLanded, err
	}
	if superseded {
		return FleetRebaseAlreadyLanded, nil
	}
	return FleetRebaseConflict, nil
}

// refSuperseded reports whether ref is already landed on base: either a strict
// ancestor (fast-forward merged) or a functional duplicate whose every commit
// already exists on base under a different sha (patch-id equivalence, robust to
// offset drift). It is the single shared supersession screen for both the
// carry-forward filter and the orphan-prune walker.
func refSuperseded(ctx context.Context, dir, ref, base string) (bool, error) {
	// is-ancestor: exit 0 → ref is reachable from base → already merged.
	if _, code, err := gitCapture(ctx, dir, "merge-base", "--is-ancestor", ref, base); err != nil {
		return false, fmt.Errorf("carryforward filter: is-ancestor %s %s: %w", ref, base, err)
	} else if code == 0 {
		return true, nil
	}
	return commitsFullyLanded(ctx, dir, ref, base)
}

// commitsFullyLanded reports whether every commit unique to ref (relative to
// base) already has a patch-id-equivalent commit on base. `git cherry base ref`
// lists each `base..ref` commit, prefixing `-` when an equivalent exists on
// base and `+` when it does not — so "no `+` lines, at least one commit" means
// the whole branch is already landed under different shas.
func commitsFullyLanded(ctx context.Context, dir, ref, base string) (bool, error) {
	out, code, err := gitCapture(ctx, dir, "cherry", base, ref)
	if err != nil {
		return false, fmt.Errorf("carryforward filter: git cherry %s %s: %w", base, ref, err)
	}
	if code != 0 {
		return false, fmt.Errorf("carryforward filter: git cherry %s %s exited %d", base, ref, code)
	}
	hasCommit, hasNew := false, false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hasCommit = true
		if strings.HasPrefix(line, "+") {
			hasNew = true
		}
	}
	return hasCommit && !hasNew, nil
}
