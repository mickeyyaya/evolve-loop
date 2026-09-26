# 2026-09-26 — a passed audit was sent back by ship-stage process issues

**Class:** pipeline, ship recovery. After a lane's audit passed, two ship-stage conditions unrelated to the audited change each discarded the verdict and re-ran earlier phases.
**Surface:** `internal/phases/ship/audit.go` (`verifyAuditBinding`) and `internal/core/ship_recovery.go` (`recoverFromShipError`).
**Found by:** the console, watching cycle 1701. The operator's directive: a change that passed audit must not be blocked by a process issue; ship must resolve it.

## What happened

Cycle 1701 (`audit-binding-report-comment-fallback-is-dead-code`) passed audit, and its ship was sent back twice before it landed as `dcfbe0c6`:

1. **`SHIP_GIT_FLEET_REBASE_NEEDED`.** Cycle 1698 had moved main first. Recovery rebased the lane cleanly, then returned to Build because the cycle carried a versioned build explanation. Build and Audit ran again for a patch that had not changed. Cycle 1698 had taken the same detour earlier that day.
2. **`SHIP_AUDIT_BINDING_TREE_MISMATCH`.** Between the re-audit and the ship, the console edited a git-tracked item in the plane's `.evolve/inbox`, appending a CI-flake recurrence. Ship's binding hashes the plane's `git diff HEAD`, so the audit was sent back again.

## Root causes

**Tree binding over the wrong tree.** `recordAuditBinding` records `tree_state_sha = sha256(git diff HEAD)` over the project root, and `verifyAuditBinding` compares it at ship. In worktree mode the plane is not what ship commits. The lane's worktree is, and the treefence check on `worktree_tree_sha` already binds it. The plane's tracked files include the inbox queue, which operators and sibling lanes move while a lane sits between audit and ship; another lane's triage claim does the same thing. Those moves had nothing to do with the audited change, yet they voided the verdict.

**A rebase always re-runs Build and Audit.** The build explanation's host binding carries the base SHA. After any rebase, `recoverFromShipError` invalidated it and returned to Build before trying the merge ladder's RUNG 0. The architect's review found that RUNG 0 could not have carried the audit even if it had run first:
- **F1.** `readCompositionSnapshot` diffs `main...<plane HEAD at audit>`, which is always empty in worktree mode. Production logs show only "snapshot unavailable", and no `composition-verdict` row exists in either ledger.
- **F2.** Even a written verdict fails ship's worktree binding: HEAD, tree-state and patch-id are all read from the plane.
- **F4.** The failed ship's own inbox consumption is in the diverged commit, so no identity proof can hold. The rebuilt 1698 and 1701 documents had to explain inbox moves.

## Fix

- **Tree binding.** `verifyAuditBinding` compares the plane-wide tree state only when the plane is the tree ship lands from; the tested root and the skip come from one decision. A worktree ship stays bound by the treefence check on the worktree it commits, so a change to the shipped worktree after audit is still refused.
- **Rebase carry-forward.** Designed in [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md) (proposed) as a four-rung, zero-LLM recovery ladder:
  - unwind the ship's bookkeeping commit;
  - rebind the explanation under a per-path byte-identity proof;
  - carry the audit forward with a fresh predicate receipt on the composed tree;
  - have ship accept the carry.

  Build and Audit re-run only when the change really differs. It lands rung by rung.

## Regression coverage

- `TestVerifyAuditBinding_PlaneBookkeepingAfterAuditDoesNotUnbindAWorktreeShip`: red before the fix with exactly 1701's error.
- `TestVerifyAuditBinding_AChangeToTheShippedWorktreeAfterAuditStillUnbinds`.
- The existing non-worktree refusals still hold: `TestVerifyAuditBinding_TreeMismatch_IntegrityError` and `TestNative_D_TreeStateMismatch_Refuses`.

## Operating rule until the fix lands

Mid-wave, the console may create new inbox items, which are untracked. It must not edit, consume or move an existing tracked item until the wave boundary.
