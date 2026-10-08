# internal/inboxstamps

> Decision: [ADR-0112](../adr/0112-loop-inbox-stamps-land-at-the-sync.md). Procedure: [runtime-reference, "The console route at a boundary"](../../operations/runtime-reference.md).

## Purpose

The loop writes into tracked inbox items in the plane when a cycle does not ship: a route to the console, a failure-count bump. `inboxstamps` decides what happens to those writes when the plane syncs with origin, so they never block the sync and are never lost silently.

## Design

- **A stamp is the mover's write, and only that.** `Classify` reads the tree's status once (`-z`, no renames) and splits it into stamps and other dirt. A modified tracked inbox item is a stamp when every key that changed between HEAD and the tree is one the mover writes (`inboxmover.IsMoverWritten`: `route` and the lifecycle-owned fields). An operator's hand edit, a deleted or added item, and any file outside the inbox are other dirt.
- **Origin is judged from the merge base.** `PlanAgainst` compares each stamped item at the merge base of HEAD and the remote with the remote's copy, so the plane's own unpublished commits never read as origin's. Whether origin touched an item is decided from its bytes (`ChangedOnRemote`, a diff from the merge base, the same rule `blockingInboxFiles` uses), because the next step is a textual merge. An item newer than the merge base, or one whose bytes origin left alone, is **kept**; one origin removed or moved is **retired**; one origin edited, even only reformatted, is **replayed**, without the fields origin itself changed, which keep origin's value; and one whose every field origin itself changed, whether it landed the stamp by hand or set its own values, is **superseded**: nothing is left to replay. Field values are compared decoded, since the mover's writer escapes `<`, `>` and `&`. An item that is not a JSON object on origin is an error, never a retirement.
- **Four steps, in order.** `Prepare` restores the retired and replayed items from HEAD, in the index and the tree. `CommitKept` commits the kept stamps alone. After the caller merges or fast-forwards, `ApplyReplay` writes the replayed fields onto origin's version through `inboxmover.UpdateItemJSON`, the mover's own writer, and `CommitReplay` commits them.
- **No failure path loses a stamp.** `Restore` re-applies the original stamps `Prepare` removed; both callers use it when a later step fails, and `evolve sync-main` reports a stamp dropped or superseded only once its merge has succeeded. `Restore` and `ApplyReplay` apply every stamp and name each one they could not apply.
- **Two callers, one rule.** `evolve sync-main` runs all four steps around its merge. The loop's wave-boundary sync runs only when nothing but stamps is dirty, never commits (a commit would make the plane diverge, which halts the loop), and leaves replayed stamps uncommitted for `evolve sync-main`.
- **A claim is loop state, not dirt.**
  - A claim moves a tracked root item into the ignored `processing/cycle-N/`, so git shows an unstaged deletion (` D`).
  - `Classify` puts such a deletion into `Claimed` (the root path and the claim path) when a claim dir holds a file of the same name. A staged deletion, a nested path, or a deletion that no claim holds stays dirt.
  - When origin edited a claimed item, `git merge` and `merge --ff-only` write origin's version back to the root. There is no conflict. This makes the duplicate of the cycle-1838 incident.
  - So after the merge (`evolve sync-main`) or the fast-forward (the wave sync), the caller runs the Mover verb `inboxmover.AbsorbRootCopies`. This package only classifies; it never moves an inbox item.
  - The Mover finds each root copy by task id, removes a copy with the same bytes, and parks a copy with other bytes in `origin-conflicts/cycle-N/`. A claim copy is never overwritten, because it can carry a lane's newer stamps ([plan](../../plans/inbox-stale-claims-2026-10.md), D8).
- **The package goes through inboxmover's facades**, never the lifecycle leaf (ADR-0079 D3).
- **Every git failure is named**, with the subcommand and its stderr.

## Tests

| Behaviour | Tests |
|---|---|
| what counts as a stamp, and what counts as a claim | `TestClassify_*` |
| kept, retired or replayed, from the merge base | `TestPlanAgainst_*` |
| landing, a staged stamp, restoring after a failure, a removal origin overrode | `TestPlan_*` |
| what origin changed since the merge base | `TestChangedOnRemote_*` |
| the git failures | `TestClassifyAndPlan_NameTheGitCommandThatFailed`, `TestClassify_ACaptureErrorIsReported` |
| the wiring | `TestSyncMain_*` and `TestSyncMainAtWaveBoundary_*` (cmd/evolve) |
