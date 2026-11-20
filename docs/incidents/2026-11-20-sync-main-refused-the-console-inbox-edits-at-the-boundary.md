# 2026-11-20: sync-main refused the console inbox edits at the boundary

Related documents: the publish procedure for console inbox changes is in the memory rule `publish_console_inbox_adds`, and the procedure of this record is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** at the wave 182 boundary, `evolve sync-main` refused with rc 3. The console had to publish its inbox changes through an inbox PR (#1593) first. Then the runtime can take main. The item says that this costs about 1 hour at each boundary.
- **Root cause:** the console edited and consumed tracked inbox files in the runtime plane. `sync-main` treats only loop-stamp fields as carried work. Each other change to a tracked file counts as dirt.
- **Fix:** the console published the changes in PR #1593. The root fix is the inbox item `sync-main-carries-console-inbox-edits`.
- **Lesson:** a verb of the console must not leave dirt that a verb of the loop refuses. The refusal is correct for code and docs. It is wrong for the console own inbox work.

## Terms used throughout

- **Loop stamp:** a field that a lifecycle verb of the loop writes to an inbox file, for example `routed_at`.
- **Dirt:** a change to a tracked file that is not a loop stamp.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| 08:15:54 | `evolve sync-main` refuses with rc 3. It names 3 tracked inbox files. | console log |
| 08:19 | The inbox PR ships as `fc1fcfa97` (PR #1593). | console log |
| 09:12 | PR #1593 lands with PR #1595. | `git log origin/main` (`ab7c9450e`, `f13d47c7d`) |

The refusal:

```text
evolve sync-main: refused — tracked files have uncommitted changes beyond the loop's inbox stamps; commit or stash first:
  .evolve/inbox/2026-10-09T00-30-00Z-inbox-claims-delta-followups.json
  .evolve/inbox/2026-11-15T14-40-27Z-shadow-contract-k5-stage-flip.json
  .evolve/inbox/2026-11-16T14-30-13Z-sync-main-refreshes-the-index-with-the-merged-binary.json
```

The first two files hold console edits (`files` and `deps`). The third is a consumed item.

## Root cause

**F1.1** `refuseDirtBeyondInboxStamps` (`go/cmd/evolve/cmd_syncmain.go:200`) returns `cliExitRefused` when `partition.Other` has an entry (`:207` to `:209`). `inboxstamps.Classify` (`go/internal/inboxstamps/inboxstamps.go:61`) puts a file in `Other` unless each changed key is loop-stamp owned (`stampBetween`, `:122`).
*Implication:* an `inbox edit` of a curated field and a `consume` are dirt.

**F1.2** The wave boundary sync has the same check (`go/cmd/evolve/cmd_loop_wavesync.go:141`).
*Implication:* `wave next` cannot start until the console publishes its inbox changes.

**F1.3** The console runs its inbox verbs in the runtime plane, and the files are tracked. No design step lets such a change ride to origin.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| The inbox changes are published through an inbox branch. | #1593 | none: a data change |
| `sync-main` carries a console route, edit or consume move through the merge. A same-field conflict keeps origin and names the dropped edit. | inbox item `sync-main-carries-console-inbox-edits` | owed by the item (its list names `TestSyncMain_CarriesAConsoleRouteEditThroughTheMerge`) |

## Limits

- That `sync-main` then succeeded is not verified in this record. The console log shows the inbox PR landed at 09:12.
- The refusal stays for code and docs dirt.

## What we learned

- Plan the inbox PR before the boundary, and keep the runtime inbox free of unpublished edits.
- A refusal that blocks each boundary by design needs a carry path, not a manual step.
