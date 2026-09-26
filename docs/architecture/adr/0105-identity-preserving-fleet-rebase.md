# ADR-0105: Identity-preserving fleet rebase — verdicts follow the change, gates follow the tree

- **Status:** Proposed, 2026-09-26. The operator's direction: "If the audit pass, we should trust the result and mitigate the ship issue and make sure the ship issue can be resolved. The pass audit changes should not be blocked because process issue."
- **Amends:**
  - [ADR-0049](0049-concurrent-multi-cycle-execution.md) S5 ("rebase and re-audit");
  - [recovery-predicate-authority](../recovery-predicate-authority.md), whose principle holds: carried predicate evidence still needs a fresh receipt on the composed tree;
  - [build-explanation-contract](../build-explanation-contract.md), where Build Binding comes to mean the authored base.
- **Record:** [incident 2026-09-26, a passed audit sent back by ship-stage process issues](../../incidents/2026-09-26-a-passed-audit-sent-back-by-ship-process-issues.md)

## Context

A fleet lane that passed audit reaches ship after a peer lane has moved main. `recoverFromShipError` rebases the lane. For every cycle with a versioned build explanation, which is every cycle today, it then returns to Build, so Build and Audit re-run. That costs 20 to 30 minutes and an LLM round each, even when the change is byte-identical. Cycles 1698 and 1701 both took this path on 2026-09-26.

The merge ladder already has a RUNG 0 meant to carry an audit across a trivial rebase. Review found it cannot carry anything in worktree mode:
- **F1.** It diffs `main...<plane HEAD at audit>`, which is always empty there. No `composition-verdict` row has ever been written.
- **F2.** Ship's binding reads HEAD, the tree state and the patch-id from the plane, so even a written verdict would not bind.
- **F3.** Explanation contract v1 puts the base SHA inside the audited patch, as the document's `## Build Binding` line.
- **F4.** The failed ship's own inbox consumption rides in the diverged commit, so no identity proof can hold as-is.
- **F5.** RUNG 0 writes its artifacts inside the worktree, which would break `ledger verify` once the worktree is removed.

## Decision

A four-rung recovery ladder with no LLM calls, as a chain of responsibility. Each rung either succeeds or falls back to today's route, which is still the safe default.

1. **B1 — unwind, rebase, re-pend** (`core/ship_recovery.go`), for contract cycles. It runs only when:
   - the worktree has no tracked changes and no untracked files;
   - the lane forked at the audited base (`merge-base HEAD main`);
   - git holds the audited tree `T0`;
   - the change since audit is exactly ship's inbox consumption pairs, and no consumed item carries a released continuation.

   The steps:
   - make a carrier commit of `T0` on the audited base (`commit-tree`), so it holds exactly the audited bytes, and reset to it;
   - replay it with the existing `rebaseWithDerivedRegen`;
   - soft-reset to the fork point, whatever the rebase's outcome. After a clean replay that is the new base; after an aborted one it is the audited base, which restores the audited shape.

   The result is the pending shape Audit binds, and the re-ship consumes its inbox items again. No unwind runs when the pre-screen predicts a conflict.
2. **B2 — `explanationdocs.RebindIdenticalRebase(ctx, binding, newBase, persist) (rebound bool, err error)`.** It rebinds the host state under an identity proof and skips Build. It returns `(false, nil)` and writes nothing on any doubt.
3. **B3 — a repaired RUNG 0.**
   - It compares tree diffs (`diff --binary --full-index --no-ext-diff --no-textconv --no-renames base0 T0` against `base1 T1`) byte-for-byte.
   - The composed-tree gates run under `treefence.Begin/End`, and the host predicate suite re-runs on `T1` with a sealed receipt.
   - It writes a `composition-verdict{method:"identical-rebase"}` record whose artifacts live in the host workspace. It skips Audit.
   - A carry always chains from the authored base and never from another carry.
4. **B4 — ship's worktree binding.** When ship lands from a worktree, it ignores the plane HEAD and the plane diff.
   - If the current worktree tree equals the audited tree, today's binding applies.
   - Otherwise it requires a carry record matching this audit's report, the audited tree, the current tree and the run. Ship then recomputes the ancestry, disjointness, byte-equal diffs, gates and receipt itself; it never trusts the writer.

### The identity proof (B2)

Patch-id is not sufficient:
- it ignores whitespace and binary content;
- `.gitattributes` textconv can hide content;
- a peer edit outside the three-line context moves the `path:line` citations the document and the audit rely on.

The proof is per-path byte identity, and every check must hold:
1. The new base is valid and different, and it descends from both the authored base and the old base.
2. **Pre-image identity:** the peer delta `D = diff --name-only(authored, base1)` and the lane's paths `S1` do not intersect, even case-folded. A `.gitattributes` or `.gitignore` in `D` declines. So does any path in `D` or `S1` that is not plain (non-ASCII; containing `:`, `\` or `~`; a component ending in a dot or a space, or longer than 250 bytes), because some filesystem resolves it to a differently spelled file.
3. **Post-image identity:** `pathStateSHA256(worktree, domain(old base), S1) == view.DiffSHA256`. This covers raw bytes, modes and the document itself.
4. The material paths and material digest are equal, and the build-report declaration still matches the view.
5. A REQUIRED document does not exist at the new base and passes its immutable-history checks.

A separate clean-replay check is not needed. A path the rebase regenerated or resolved is one the peer delta also touched, which check 2 declines, and any other byte change fails check 3.

### What the host may re-derive

| Artifact | Re-derived | Must stay equal |
|---|---|---|
| Activation marker | `BaseSHA` → new base | contract, cycle, run, workspace, worktree |
| Result snapshot | `View.BaseSHA`, `View.DiffSHA256`, a new `View.AuthoredBaseSHA` (set once) | material digest, status, reason, document path and digest, material paths |
| `build-explanation.json` | rewritten to the new view | must satisfy `SameView` |
| The Markdown document, `build-report.md` | nothing | bytes (proven) |

`BaseSHA`, `DiffSHA256` and the handoff are host-derived facts. The Builder's document keeps saying it was written against the authored base, which stays true, so no new rationale is asserted over proven-identical bytes. `Verify` becomes lineage-aware: the document's base must equal `AuthoredBaseSHA` when it is set, with ancestry and disjointness re-derived; when it is absent, strict equality applies. `SealResult` never sets it.

### Placement

In `ship_recovery.go`, after the pre-screen: B1, then the rebase.
- If the change is pending on its fork point and identical, B2 runs. When it rebinds, B3 runs and returns Ship on a green carry, else Audit. A committed change never goes to Audit this way: the Auditor reads `git diff HEAD`, which would be empty.
- Otherwise today's `RebaseBuildAndPersist` returns Build.

The write order is marker, then `persist`, then the handoff, then the snapshot, which is the commit point. A carry record is written only after the snapshot.

## Consequences

- A clean fleet rebase of an unchanged change ships with no LLM round. Build re-runs only when the change differs, and Audit only when the composed tree fails its gates or predicates.
- About six protected files change: `explanationdocs`, `phaseio/handoffs.go`, `core/ship_recovery.go` and the composition carry-forward, `phases/ship/audit.go` and `composition.go`, and the ledger composition adapter. The ADR-0064 manifest gains the ship binding reader and the composition files, which are not protected today.
- An old binary ignores `authored_base_sha` and fails closed. Deploy at a wave boundary.

## Components, steps and tests

Each rung lands at its own wave boundary as a stack of small commits, one component per commit. Every component has its own tests, which run alone with `go test -count=1 -run '<selector>' ./internal/<pkg>/`. A component lands unwired first; the wiring is a separate commit that carries the integration tests. The suite is green at every commit, and the full floor runs on the tip.

### B2 — the explanation rebind (`internal/explanationdocs`)

| Component | Step it implements | Tests |
|---|---|---|
| Path states: `readPathStates`, `foldPathStates` | one read of the lane's paths, folded per digest domain | the existing digest suite |
| Authored base: `AuthoredBaseSHA`, lineage-aware `Verify` | Audit, Ship and Retro accept the document's base only with lineage re-derived | `TestVerify_` |
| Re-seal policy: `SealResult`, `RefreshResult`, `revalidateResult` | the host never re-seals a rebound handoff | `TestSealResult_`, `TestRefreshResult_` |
| Alias guard: `peerTouchesLane`, `isPlainPath` | the disjointness check | `TestPeerTouchesLane` |
| `RebindIdenticalRebase` | the proof, then marker → checkpoint → handoff → snapshot | `TestRebindIdenticalRebase_` |

Content drift is covered by the whitespace and binary drift tests. The check that the document is absent at the new base is defence in depth: disjointness and the digest decline that case first, so no public seam reaches it.

### B1 — the unwind, then the wiring (`internal/core`), in commit order

| # | Component | Step it implements | Tests |
|---|---|---|---|
| 1 | `forkPoint` (a fix) | the rebased base is the lane's fork point, `merge-base HEAD main`, not main's tip, which a later landing moves | `TestRouteRebasedExplanation_BindsTheForkPoint` |
| 2 | `latestAuditedTree` | read the audited tree `T0` from the newest auditor row of the run, by the same rule ship binds (`auditledger`); an empty tree declines | `TestLatestAuditedTree_` |
| 3 | `unwindShipCommit`, `pendRebasedChange` | replace ship's commit with a carrier of `T0` on the audited base, then leave the change pending on the fork point; it declines unless the delta is exactly ship's inbox consumption pairs, the worktree has no untracked files, and no consumed item carries a released continuation | `TestUnwindShipCommit_`, `TestPendRebasedChange_` |
| 4 | `routeRebasedExplanation` calls B2 | only a change pending on its fork point may be rebound, because Audit reads `git diff HEAD`; the result is Audit, anything else Build, and an incomplete rebind aborts | `TestRouteRebasedExplanation_` |
| 5 | Wiring: unwind → rebase → pend → route | contract cycles only; no unwind when the pre-screen predicts a conflict; pend whatever the rebase's outcome, so a failed rebase leaves the audited shape | `TestRecoverFromShipError_IdenticalRebaseSkipsBuild`, `…_NonIdenticalRebaseStillReturnsToBuild`, `…_WithoutAnAuditedTreeRebasesTheShipCommit`, `…_APredictedConflictIsNotUnwound`, `…_ALegacyCycleIsNotUnwound` |

B1 opens two crash windows. After the carrier reset and before the rebase, a resumed ship binds `T0` again and the unwind repeats, so that window heals itself. Between the rebase and the pend, the carrier sits committed on the new base, and a resumed ship would go to a re-audit of a committed change. The resume heal (`resume-heals-a-carrier-left-at-head`, via the `Evolve-Carrier` trailer) is therefore required before B3 or B4 builds on B1. Before building B3, confirm that the first live fleet-rebase recovery logs `unwound its ship commit`, not `ship unwind declined:`.

### Phase by phase

- **Build.** A change that is not identical still invalidates the snapshot and returns to Build (step 5's non-identical test).
- **Audit.** The rebound explanation verifies on the new base (`explanationdocs.Verify` in steps 4 and 5), and Audit always receives a pending change.
- **Ship.** B4's `TestFleetRebase_Cycle1701Shape_ShipsWithoutBuildOrAudit` drives ship → recovery → ship with no Build or Audit round.

### A constraint on B4 found on the way

Ship's comparison of the plane's HEAD is load-bearing for a worktree ship: a resumed ship detects "merged locally, push pending" through it (`phases/ship/repair_resume_test.go`). Dropping the check for worktree ships turns that resume into a false "shipped". B4 must keep that detection, for example by asking whether the plane already contains the lane's tip, before it stops treating a sibling's closeout commit as a moved HEAD (cycle 1704, 2026-09-26).

B3 and B4 get their own tables when they are built, in the same shape.
## Rollout, rung by rung, each test-first and merged at a boundary

0. **Done.** The worktree-mode tree binding: ship binds a worktree ship to the worktree it lands from, not to the plane's bookkeeping. This removes 1701's second bounce.
1. **Done.** B2 with its `Verify` lineage and the tests in its component table above (`rebind_identical_rebase_test.go`). Nothing calls it until B1 wires the recovery.
2. **Done.** B1 (the unwind) and the reordered recovery, returning Audit, not Build, when the change is identical.
3. B3 (the repaired RUNG 0) together with the F5 artifact relocation.
4. B4 (ship accepts a carry) and the end-to-end test `TestFleetRebase_Cycle1701Shape_ShipsWithoutBuildOrAudit`.

The evidence, the full test list (33 named tests), the crash windows and the mutation map are in the [design review](../../research/2026-09-26-identity-preserving-rebase-design-review.md).
