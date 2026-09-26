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

1. **B1 — unwind, rebase, re-pend** (`core/ship_recovery.go`). Only when the worktree is clean, the audited base is an ancestor of HEAD, and every post-audit change is under `.evolve/inbox/`:
   - soft-reset to the audited base;
   - restore the audited tree `T0` (checking `write-tree == T0`);
   - make a carrier commit and replay it with the existing `rebaseWithDerivedRegen`;
   - soft-reset to main.

   The result is the pending shape audit binds, and the re-ship consumes its inbox items again.
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
1. The rebase replayed cleanly with no derived-file regeneration.
2. The new base is valid and different, and it descends from both the authored base and the old base.
3. **Pre-image identity:** the peer delta `D = diff --name-only(authored, base1)` and the lane's paths `S1` do not intersect, even case-folded. A `.gitattributes` or `.gitignore` in `D` declines.
4. **Post-image identity:** `pathStateSHA256(worktree, domain(old base), S1) == view.DiffSHA256`. This covers raw bytes, modes and the document itself.
5. The material paths and material digest are equal, and the build-report declaration still matches the view.
6. A REQUIRED document does not exist at the new base and passes its immutable-history checks.

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
- If the change is identical, B2 runs. When it rebinds, B3 runs and returns Ship on a green carry, else Audit.
- If the change is not identical, today's `RebaseBuildAndPersist` returns Build.

The write order is marker, then `persist`, then the handoff, then the snapshot, which is the commit point. A carry record is written only after the snapshot.

## Consequences

- A clean fleet rebase of an unchanged change ships with no LLM round. Build re-runs only when the change differs, and Audit only when the composed tree fails its gates or predicates.
- About six protected files change: `explanationdocs`, `phaseio/handoffs.go`, `core/ship_recovery.go` and the composition carry-forward, `phases/ship/audit.go` and `composition.go`, and the ledger composition adapter. The ADR-0064 manifest gains the ship binding reader and the composition files, which are not protected today.
- An old binary ignores `authored_base_sha` and fails closed. Deploy at a wave boundary.

## Rollout, rung by rung, each test-first and merged at a boundary

0. **Done.** The worktree-mode tree binding: ship binds a worktree ship to the worktree it lands from, not to the plane's bookkeeping. This removes 1701's second bounce.
1. B2 with its `Verify` lineage and tests 1–16 of the design list.
2. B1 (the unwind) and the reordered recovery, returning Audit, not Build, when the change is identical.
3. B3 (the repaired RUNG 0) together with the F5 artifact relocation.
4. B4 (ship accepts a carry) and the end-to-end test `TestFleetRebase_Cycle1701Shape_ShipsWithoutBuildOrAudit`.

The evidence, the full test list (33 named tests), the crash windows and the mutation map are in the [design review](../../research/2026-09-26-identity-preserving-rebase-design-review.md).
