---
score_cap:
  - criterion: "A superseded ref checked out in a worktree is kept as kept-checked-out with no `git branch -D` issued, and the walk continues to later refs"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1773_001_' ./acs/cycle1773/"
  - criterion: "A superseded ref a continuation binding names is never pruned (kept-bound, no delete, no PR check), and a ref both checked out and bound carries exactly one kept reason"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1773_002_' ./acs/cycle1773/"
  - criterion: "Any other refused `git branch -D` is reported per ref as its own reason (kept-delete-failed), never folded into checked-out or open-PR, and never aborts the rest of the walk"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1773_003_' ./acs/cycle1773/"
  - criterion: "When a keep cannot be decided (worktree list fails, continuation registry unreadable, hasOpenPR errors) the walk aborts before any delete"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1773_004_' ./acs/cycle1773/"
  - criterion: "`evolve branches prune` prints kept-checked-out / kept-bound / kept-delete-failed per ref against real git (never kept-open-pr for them), in both --dry-run=false and dry-run"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1773_005_' ./acs/cycle1773/"
---

# Eval: `evolve branches prune` keeps checked-out and continuation-bound refs and reports why

> Pins `PruneSupersededOrphans` (`go/internal/core/prune_superseded_orphans.go`)
> and its CLI surface `runBranchesPrune` (`go/cmd/evolve/cmd_branches.go`).
> A superseded `cycle-*` ref that is checked out in a worktree, or that a
> continuation-registry binding names, must be kept before any PR check or
> `git branch -D`. Each kept ref gets its own reason on the per-ref line, and
> the rest of the walk continues. Source: the 2026-09-29 storage cleanup, where
> 50 superseded refs included the live lane 1762 and the kept failed cycle 1757
> (inbox item `branches-prune-aborts-on-checked-out-branch`). Cycle 1773's
> first build fixed only the abort. Its audit (H1/H2/M1) found that bound refs
> were still deleted, and that every refused delete was labeled `kept-open-pr`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| checked-out-keep | a worktree-held ref is kept-checked-out, no delete, walk continues | 8/10 | `go test -tags acs -run '^TestC1773_001_' ./acs/cycle1773/` |
| bound-keep | a continuation-bound ref is never pruned; one kept reason per ref | 9/10 | `go test -tags acs -run '^TestC1773_002_' ./acs/cycle1773/` |
| delete-failed | a residual refused delete is kept-delete-failed per ref, not silent | 6/10 | `go test -tags acs -run '^TestC1773_003_' ./acs/cycle1773/` |
| fail-closed | an undecidable keep aborts before any delete | 7/10 | `go test -tags acs -run '^TestC1773_004_' ./acs/cycle1773/` |
| cli-labels | the operator sees the true reason per ref, real git | 8/10 | `go test -tags acs -run '^TestC1773_005_' ./acs/cycle1773/` |
