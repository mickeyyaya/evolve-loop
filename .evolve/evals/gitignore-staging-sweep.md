---
score_cap:
  - criterion: "Unrelated untracked residue is absent from the worktree content tree the audit binding and the verdict cache are keyed on"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestWorktreeContentSHA_ExcludesUnrelatedUntrackedResidue$' ./internal/core"
  - criterion: "A new file the builder declared by staging it is retained in the content tree"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestWorktreeContentSHA_StagesDeclaredNewFile$' ./internal/core"
  - criterion: "An unstaged modification to a tracked file is still captured, so a dirty lane never binds a base-identical identity"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestWorktreeContentSHA_CapturesUnstagedTrackedModification$' ./internal/core"
  - criterion: "A worktree whose only difference from its base is residue keeps its base tree identity exactly"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestWorktreeContentSHA_ResidueOnlyWorktreeKeepsBaseIdentity$' ./internal/core"
  - criterion: "The production audit-binding path (emitPhaseBindings to recordAuditBinding) records the scoped tree, not a git add -A tree"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestEmitPhaseBindings_AuditBindingTreeExcludesUnrelatedResidue$' ./internal/core"
---

# Eval: Scope the binding content tree to declared work

> Pins the staging boundary of `worktreeContentSHA`
> (`go/internal/core/phase_bindings.go`), the single source for both the audit
> binding's `WorktreeTreeSHA` (`phase_bindings.go:129`) and the ADR-0048 Slice B
> verdict-cache key. Before cycle 1594 it staged the whole worktree with
> `git add -A`, so any untracked file that happened to sit in the lane — a
> bug-reproduction reproducer, a regenerated `go/coverage.*.txt` artifact, a
> minted phase stub, another agent's scratch file — was adopted into the tree
> the auditor's binding attests and into the cache key later cycles look up. The
> binding therefore certified content no auditor reviewed. Source incidents:
> the wave-3 cycle-1572 and cycle-1574 staging rejections, amortised into the
> `gitignore-staging-sweep` inbox record (14 auditor prescriptions,
> `.evolve/inbox/2026-08-27T21-00-00Z-gitignore-staging-sweep.json`), and the
> earlier staging contract
> `.evolve/inbox/2026-07-20T18-12-00Z-ship-addall-staging-surface.json`.
> Reproduced live in cycle 1594
> (`.evolve/runs/cycle-1594/bug-reproduction-report.md`).
>
> The boundary is expressed in observable Git terms so it survives
> re-implementation: declared content is what the lane's index already holds
> (tracked files plus anything the builder explicitly staged) together with
> tracked modifications on disk; undeclared content is an untracked file nobody
> staged. Two of the five caps are deliberately over-correction guards — the
> cheapest way to exclude residue is to bind `HEAD^{tree}` or stage nothing at
> all, which drops real builder output and manufactures base-identical
> identities (the INTEGRITY_TREE_DRIFT class of cycle-152 and the fresh-base
> verdict-cache collision `verdictcache.ProbeEligible` exists to prevent).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| residue-exclusion | Undeclared untracked residue never enters the content tree | 9/10 | `go test -run TestWorktreeContentSHA_ExcludesUnrelatedUntrackedResidue ./internal/core` |
| declared-retention | Staged new builder output survives the scoping | 8/10 | `go test -run TestWorktreeContentSHA_StagesDeclaredNewFile ./internal/core` |
| tracked-capture | Unstaged tracked edits are still captured (anti-degenerate) | 8/10 | `go test -run TestWorktreeContentSHA_CapturesUnstagedTrackedModification ./internal/core` |
| empty-selection-edge | A residue-only lane keeps its base identity | 7/10 | `go test -run TestWorktreeContentSHA_ResidueOnlyWorktreeKeepsBaseIdentity ./internal/core` |
| production-wiring | The real audit-binding path emits the scoped tree | 9/10 | `go test -run TestEmitPhaseBindings_AuditBindingTreeExcludesUnrelatedResidue ./internal/core` |
