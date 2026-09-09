# Continuation adoption boundary

Cycles 1607 and 1609 exposed an ordering defect during live recovery verification:
archiving unpublished explanation records staged changes before the snapshot was
merged with current main. Git refused that dirty merge; adoption still dispatched
later phases on the old base. The pre-adoption clean-merge screen did not protect
against changes introduced by adoption itself.

The native adoption sequence is now:

1. Validate the preserved immutable snapshot and its original ancestry. An invalid
   candidate leaves fresh provisioning intact and releases its declined binding.
2. Seed the current cycle worktree from that snapshot. Once replacement starts,
   errors stop the cycle; the caller cannot assume the fresh worktree survived.
3. Resolve local main to a commit SHA, then merge that exact SHA while the seeded
   worktree is clean. Return that SHA even when it is already an ancestor.
4. Archive unpublished explanation records relative to the effective main base.
   Canonical records already published on main remain canonical. Archive collisions
   are explicit failures; existing records are never overwritten.
5. Persist the effective review base and continuation manifest before further
   phase dispatch. Preserve the ancestor snapshot and any seeded work on failure.

The wave controller remains responsible for synchronizing local main before
launching lanes. A moving ref cannot change the recorded adoption base after its
SHA is resolved. A raced merge conflict is aborted and surfaced through the native
abnormal closeout; it cannot downgrade into successful stale-base adoption.

`TestRunCycle_AdoptsContinuationAndServesFindings` composes real Git snapshots,
main advancement, archive handling, persisted review-base projection and Build
handoff. The integration-tagged
`TestRunCycle_ContinuationAdvanceFailureStopsBeforeBuild` proves raced merge and
seeding failures stop TDD/Build/Audit/Ship and preserve the available work.

The [live validation record](../reports/2026-09-09-recovery-validation.md) records
quota interruption separately. This boundary does not add fleet-cohort resume or
change checkpoint discovery eligibility.
