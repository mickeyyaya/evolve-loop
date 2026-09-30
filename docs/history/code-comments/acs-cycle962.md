# Comment history: `acs/cycle962`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle962/predicates_test.go:3` — above `package cycle962`

```text
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
```

### `go/acs/cycle962/predicates_test.go:208` — above `git(t, dir, "checkout", "-q", "-b", "cycle-100")`

```text
// cycle-100: change already landed on main (patch-id dup) → superseded.
```

### `go/acs/cycle962/predicates_test.go:213` — above `git(t, dir, "checkout", "-q", "main")`

```text
// cycle-101: distinct work NOT on main → not superseded.
```

### `go/acs/cycle962/predicates_test.go:218` — above `git(t, dir, "checkout", "-q", "main")`

```text
// main absorbs cycle-100's change under a new sha.
```
