---
score_cap:
  - criterion: "A reused worktree whose HEAD is a salvage snapshot records the first non-salvage ancestor as the worktree base, and the salvaged work is pending in the review diff after normalize"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestWorktreeReuseBase_(SalvageSnapshotHEADResolvesToFirstNonSalvageAncestor|SalvagedWorkIsPendingInTheReviewDiffAfterNormalize)' ./internal/core"
  - criterion: "Stacked salvage snapshots resolve past ALL of them, not one hop"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestWorktreeReuseBase_StackedSalvageSnapshotsResolveToTheCommitBeneathAll ./internal/core"
  - criterion: "An ordinary reused HEAD is recorded verbatim; the marker is classified from the commit SUBJECT, never a substring anywhere in the message"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestWorktreeReuseBase_(OrdinaryHEADIsRecordedVerbatim|CommitMentioningSalvageInBodyIsNotTreatedAsASnapshot)' ./internal/core"
  - criterion: "An unresolvable ancestor degrades with a loud WARN naming the snapshot, never silently"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestWorktreeReuseBase_UnresolvableAncestorWarnsLoudlyAndDoesNotDegradeSilently ./internal/core"
  - criterion: "The guard is reached from the production provisioning path (orchestrator Create + base capture), not only from a helper-level test"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestWorktreeReuseBase_ProvisioningPathRecordsGuardedBaseInCycleState ./internal/core"
---

# Eval: Guard Create-reuse against a salvage snapshot becoming the normalize base

> ADR-0076 slice C changed preserved-worktree state from dirty to COMMITTED: a FAILed
> cycle's work is snapshot-committed onto its cycle branch under the subject
> `salvage snapshot (ADR-0076 continuation-on-fail)`. The ADOPTION path (CreateFrom)
> knows that shape; the REUSE path in `gitWorktree.Create` does not. When Create
> reuses an existing worktree for the same cycle number (resume/reset edge), the tree
> is now clean, so `ensureCleanWorktree` no-ops and the base capture in
> `cyclerun.go` (`rev-parse HEAD`) records the SALVAGE SNAPSHOT as
> `CycleState.WorktreeBaseSHA`. `normalizeWorktreeToBase` then soft-resets to that
> same snapshot: the salvaged work sits AT the base, the review diff is empty, and the
> audit is told the builder produced nothing. This eval pins the guard permanently so
> a later refactor of the reuse path cannot silently reintroduce it. Source: cycle-1546
> (architect finding #3 of the ADR-0076 slice C review, inbox
> `continuation-create-reuse-snapshot-base-guard`); the same worktree-base
> classification limb was implicated in the cycle-1542 / cycle-1544 audit FAILs.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| snapshot-base-resolution | Salvage-snapshot HEAD ⇒ base is the first non-salvage ancestor AND the salvaged work is pending for audit | 8/10 | `go test -run 'TestWorktreeReuseBase_(SalvageSnapshotHEADResolvesToFirstNonSalvageAncestor\|SalvagedWorkIsPendingInTheReviewDiffAfterNormalize)' ./internal/core` |
| stacked-snapshot-edge | Two or more stacked salvage snapshots resolve past all of them (no single-hop `HEAD^`) | 6/10 | `go test -run TestWorktreeReuseBase_StackedSalvageSnapshotsResolveToTheCommitBeneathAll ./internal/core` |
| no-over-reach-negative | Ordinary HEAD recorded verbatim; subject-line classification, not substring | 7/10 | `go test -run 'TestWorktreeReuseBase_(OrdinaryHEADIsRecordedVerbatim\|CommitMentioningSalvageInBodyIsNotTreatedAsASnapshot)' ./internal/core` |
| loud-degrade | Unresolvable ancestor WARNs naming the snapshot instead of degrading silently | 6/10 | `go test -run TestWorktreeReuseBase_UnresolvableAncestorWarnsLoudlyAndDoesNotDegradeSilently ./internal/core` |
| wiring-proof | Guard reached from the real provisioning path, not only a helper-level test | 8/10 | `go test -run TestWorktreeReuseBase_ProvisioningPathRecordsGuardedBaseInCycleState ./internal/core` |
