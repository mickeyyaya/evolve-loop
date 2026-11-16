# 2026-11-14: shipped work stayed open in the inbox, and lanes and design seats spent cycles on it

Related documents: the package note [internal-inboxcurator.md](../architecture/packages/internal-inboxcurator.md).

## Summary

- **Impact:** of 14 top-ranked console items that design seats examined on 2026-11-14 and 2026-11-15, 8 (57%) were already done on main. In wave 173, cycles 2380 and 2381 claimed shipped items and sealed `SKIPPED_UNKNOWN`.
- **Scale:** on origin/main, 11 of 251 open items are named by a `Closes-Inbox:` trailer of a landed commit. Two examples are `87e2e1673` and `706c1d327`, both of 2026-11-04.
- **Root cause:** the deterministic `Closes-Inbox` consume reads only the commit range between two cursor positions. The cursor started at `HEAD`, so a trailer that landed before it is never read again. The sweep rule of the curator is a judgment rule only, and `enforce` does not apply a judgment rule.
- **Fix:** PR #1468 (fix D, two design rounds) adds one shared predicate and a standing sweep. On this repository, `evolve inbox curate` then proposes 11 deterministic consumes, where it proposed 0 before.
- **Status of this record:** the fix is in PR #1468. The predicate `closedBy` is not on main, so the "What changed" section marks it as a design.

## Terms used throughout

- **Consume:** the move of an inbox item from the open set to `.evolve/inbox/consumed`.
- **Trailer:** a `Closes-Inbox: <id>` line at the end of a commit message. It names the item that the commit closes.
- **Cursor:** the commit position at which the curator last read the history.
- **Sweep:** a pass of the curator over all open items, not only over the new commits.
- **Design seat:** a phase agent that examines an item and writes a design for it.

## Timeline

| Time | Event | Evidence |
|---|---|---|
| 2026-11-04 | Commits `87e2e1673` and `706c1d327` land with a `Closes-Inbox:` trailer. The curator does not read these trailers later. | `git log -1 87e2e1673`, `git log -1 706c1d327` |
| Wave 173 | Cycles 2380 and 2381 claim items that are already shipped. Both seal `SKIPPED_UNKNOWN`. | wave 173 record |
| 2026-11-14 and 2026-11-15 | Design seats examine 14 top-ranked console items. 8 of them are already done on main. | the 8 items in F1.4 |
| 2026-11-14 | PR #1468 (fix D) is designed in two rounds. | PR #1468 |

The fact sheet gives no clock time for these events.

## Root cause

**F1.1** `matchOne` makes a consume proposal when the change names the item in `Closes-Inbox` (`go/internal/inboxcurator/landing.go:110`, the check `slices.Contains(c.Closes, it.ID)`).
*Implication:* the deterministic consume exists, but it takes a `Change` as input.

**F1.2** The curator gives `matchOne` only the commits in the cursor range at each boundary. The cursor started at `HEAD`.
*Implication:* a trailer that landed before the cursor (for example through a manual console ship before the train existed) is never read again.

**F1.3** The sweep rule of the curator says an item is consumed when "N of N named tests exist". That rule is judgment only. `enforce` does not apply it.
*Implication:* no other path closes the item, so the item stays open.

**F1.4** Eight of the 14 examined items were already done on main: `train-record-pr-has-no-reachable-attestation`, `gate-contract-g4-base-red-attribution`, `role-guard-derives-the-active-cycle`, `wave-integration-i13-line-lists-union`, `landed-identity-zero-context`, `resume-path-has-no-blocker-route`, `builder-fence-erases-new-files-and-the-seat-says-pass-ac-1-console` and `build-floor-keeps-audit-rules`.
*Implication:* the rank puts these items at the top, so each lane and each design seat takes them first.

**F1.5** Wave 173 cycles 2380 and 2381 took shipped items and sealed `SKIPPED_UNKNOWN`.
*Implication:* each open shipped item costs a full cycle with no ship.

## What changed

PR #1468 holds the design. These points come from the fact sheet. I did not check them against the code of the PR.

| Change | Regression test |
|---|---|
| One predicate `closedBy`: the id matches and the commit is not older than the item. `matchOne` and a standing sweep share it. | not given in the fact sheet |
| `Inputs.History` is read only by `ClosedOnMain` in the Backstop block. History never feeds the judgment rules. | not given in the fact sheet |
| A closes-only read with a `git log --grep` pre-filter. | not given in the fact sheet |

Measured result of the design on this repository: `evolve inbox curate` proposes 11 deterministic consumes (0 before). The wall time goes from 0.80 s to 0.90 s.

On main at the time of this record, `closedBy` and `ClosedOnMain` are not in `go/internal/inboxcurator/`. The fix has not landed there.

## What we learned

- A consume that reads only a cursor range loses each fact that landed before the cursor. A standing sweep must read the history again.
- A rule that only a judgment seat applies cannot close an item without a person. A fact that a commit states (the trailer) must close the item by a deterministic rule.
- One predicate must serve the landing path and the sweep path. Two copies of the rule can disagree.
- History must not feed the judgment rules. Only the Backstop block reads it.
