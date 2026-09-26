# Design review — identity-preserving fleet rebase (ADR-0105), 2026-09-26

An architect's read-only review for [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md), produced after cycles 1698 and 1701 re-ran Build and Audit for unchanged changes. This page keeps the parts the ADR summarises: the evidence behind F1–F5, the test list and the mutation map.

## Evidence for the premise corrections

- **F1 (RUNG 0 never fires in worktree mode).** `cmd_composition_wiring.go` `readCompositionSnapshot` diffs `main...entry.GitHEAD`, and `entry.GitHEAD` is the plane HEAD at audit time (`phase_bindings.go` `recordAuditBinding`). Runtime logs from 2026-08-29 and 2026-08-31 show only "snapshot unavailable: git patch-id --stable: empty output", and neither ledger holds a `composition-verdict` row. Under contract v1, `ship_recovery.go` returns Build before RUNG 0 is reached.
- **F2 (a verdict would not bind).**
  - `adapters/ledger/composition.go` matches `GitHead` against the plane HEAD, while the writer records the worktree HEAD.
  - It compares a write-tree id with sha256 of the plane `git diff HEAD`.
  - It recomputes the patch-id from the plane diff.
  - The treefence and the predicate receipt refuse any tree but the audited one, by design (`recovery-predicate-authority.md`).
- **F3 (base inside the patch).** `explanationdocs` requires the document's `## Build Binding` to equal `binding.BaseSHA`, and the document is part of the diff.
- **F4 (bookkeeping in the diverged commit).** `worktree_ship.go` consumes inbox items before the worktree commit; `.evolve/inbox/**` is material, and consumed copies are re-serialized with timestamps. The rebuilt 1698 and 1701 documents had to explain inbox moves.
- **F5 (artifacts in the worktree).** RUNG 0 writes its diff artifacts under the worktree, and `ledger verify` treats an unreadable artifact as a broken chain.

## Test list

### explanationdocs (real git fixtures)

1. `TestRebindIdenticalRebase_DisjointCleanRebaseRebindsWithoutTouchingBuilderFiles`
2. `…_PeerTouchedALanePathDeclines`: a same-file, non-overlapping hunk whose patch-id is equal.
3. `…_WhitespaceOnlyDriftDeclines`
4. `…_BinaryContentDriftDeclines`
5. `…_ExtraShipConsumptionPathDeclines`: F4 without B1.
6. `…_ContentDriftDeclines`
7. `…_NonDescendantBaseDeclines`
8. `…_DocumentPresentAtNewBaseDeclines`
9. `…_GitattributesInPeerDeltaDeclines`
10. `…_CaseFoldCollisionDeclines`
11. `…_SecondRebindKeepsAuthoredBase`
12. `…_WriteOrderLeavesRecoverableSplit`
13. `…_NotApplicableHandoff`
14. `TestVerify_AbsentAuthoredBaseKeepsStrictEquality`
15. `TestVerify_AuthoredBaseMustBeAncestorAndDisjoint`
16. `TestSealResult_NeverWritesAuthoredBase`

### core

17. `TestRecoverFromShipError_IdenticalRebaseSkipsBuild`: rewrites `…_RebaseInvalidatesExplanationAndReturnsToBuild`.
18. `…_NonIdenticalRebaseStillReturnsToBuild`
19. `…_IdenticalRebaseGreenCarryReturnsShip`
20. `…_RebindThenRedGatesReturnsAudit`
21. `…_LedgerWriteFailureReturnsAudit`
22. `TestUnwindShipCommit_RestoresInboxAndPendsT1`
23. `…_NonInboxPostAuditDeltaDeclines`
24. `…_DirtyWorktreeDeclines`
25. `TestCompositionSnapshot_WorktreeModeIsNonEmpty`: red today, pinning F1.
26. `TestFleetRebase_Cycle1701Shape_ShipsWithoutBuildOrAudit`: integration.

### ship

27. `TestVerifyAuditBinding_WorktreeCarryBindsComposedTree`
28. `…_CarryWithoutFreshReceiptRefused`
29. `…_CarryKernelRecomputesIdentity`: a forged record is refused.
30. `…_CarryChainedOnCarryRefused`
31. `…_ForeignAuditOrRunRefused`
32. `…_RedGateRefused`
33. `TestVerifyPostPushPredicateEvidence_AcceptsCarriedTree`

Add `RebindIdenticalRebase` to the explanation call-site vocabulary floor in `integrity_surface_explanation_callsites_test.go`.

## Mutation map

| Mutation | Killed by |
|---|---|
| Drop the disjointness check | 2, 11 |
| Swap the digest for patch-id | 3, 4 |
| Drop the digest check | 5, 6 |
| Drop the ancestry check | 7 |
| Drop the check that the document is absent at the new base | 8 |
| Write the snapshot before the marker | 12 |
| Accept an absent authored base permissively | 14 |
| Skip the B1 unwind | 26 |
| Skip the fresh receipt at ship | 28 |
| Trust the writer's identity claim | 29 |
| Allow carry chaining | 30 |

## Crash windows

| Crash after | State left | Recovery |
|---|---|---|
| The marker write | marker = new base, cycle state and snapshot = old | `RecoverRebaseSplit` recognises it and routes to Build |
| `persist` or the handoff write | marker = cycle state = new, snapshot = old | recoverable, routes to Build |
| The snapshot write | committed | atomic rename, so no partial file |

- The snapshot is written before the carry record. A committed rebind with no record reaches ship with a tree mismatch and routes to Audit, not Build. A stale record is inert, because matching requires the composed tree to equal the current one.
- A pre-existing window (W0) remains: git has rebased while host state is still on the old base. The review recommends a git-aware split heal on resume.

## Risks

1. The blast radius spans about six protected files across four packages; this is console work, merged only at wave boundaries.
2. A semantic interaction outside the lane's files is accepted under the operator directive, following Gerrit's TRIVIAL_REBASE precedent, and bounded by the disjointness rule.
3. The gate runtime on a carry is minutes of CPU, still far below an LLM round.
4. F5 must land together with B3.
5. B1 changes the Build fallback too: rebuilt documents stop explaining inbox moves.
6. B4 stops refusing a moved plane HEAD in worktree mode. This is a separate behaviour change and needs its own review.
7. A reviewer may be confused when the document's base differs from the handoff's; render the lineage in the handoff.
