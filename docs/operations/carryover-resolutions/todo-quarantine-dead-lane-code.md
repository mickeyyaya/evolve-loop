# Carryover resolution — `todo-quarantine-dead-lane-code`

- **Entry:** `.evolve/state.json:carryoverTodos[id=todo-quarantine-dead-lane-code]`
- **First seen:** cycle 1159 · **cycles_unpicked:** 3 · **Resolved:** cycle 1181
- **Verdict:** **NO-OP — no `QUARANTINED-DEAD` marker is warranted on either lane.**

## What the entry asked

> Mark whichever of `carryforward-filter-wire-fleet-rebase` / `menu-pass-preserve-committed-ids`
> did not land cycle 1159 with a `QUARANTINED-DEAD` doc marker (or confirm no-op if both landed).

## Verdict: both lanes landed, so the answer is the no-op branch

### `menu-pass-preserve-committed-ids` — LANDED

`go/internal/triagecap/lane_menu.go:102-107` (`SelectWaveSeedMenus`) routes a non-empty
`committed` prefix through the committed-**aware** `WidenTopNToFleetWidth` instead of the
committed-**blind** `SelectFleetWidthTopN` that used to drop or reorder a committed id behind a
higher-weight backlog item. The same function calls `pruneConsumed` first, so preserving committed
ids did not degrade into preserving *consumed* ones (the cycle-1116 re-pin). Behaviourally pinned
this cycle by `go/acs/cycle1181/predicates_test.go`
(`TestC1181_002_.../menu_pass_preserves_committed_ids`), which drives the real function over an
isolated temp tree — a low-weight committed id must survive the seed and a consumed one must not.

### `carryforward-filter-wire-fleet-rebase` — LANDED

`go/internal/core/carryforward_filter.go:66-113` exports `FleetRebaseVerdict` with
`FleetRebaseAlreadyLanded` / `FleetRebaseConflict` and the `ClassifyFleetRebaseCandidate` pre-screen.
Commit `9eacd83f` (`test(core): name+exercise fleet-rebase classify surface — fixes repo-wide
apicover RED on main`) named and exercised that surface on `main`, which is what closed the export
gap the entry was worried about. Pinned this cycle by
`TestC1181_002_.../fleet_rebase_classifier_is_wired`: a cherry-picked candidate must classify
`FleetRebaseAlreadyLanded` over a real git tree, and — the load-bearing negative — a genuine 3-way
conflict must classify `FleetRebaseConflict`, never `AlreadyLanded`.

Because the verdict is no-op, neither source file carries (or may gain) a `QUARANTINED-DEAD`
marker; marking live, wired code dead is the inverse defect. `TestC1181_001_...` asserts that
absence directly.

## The entry's either/or framing is itself stale

The entry is phrased as an **either/or** — "whichever … did not land" — which presupposes that
exactly one of the two lanes died at cycle 1159. That presupposition is false: **neither** branch is
dead. A future reader meeting this question should not re-derive the dead end; the correct action
was always "confirm no-op and retire the entry", and it is done.

## Retirement of the live entry

Removed from the live `.evolve/state.json:carryoverTodos` via the sanctioned locked
read-modify-write path (`evolve carryover apply-decisions --apply`,
`go/cmd/evolve/cmd_carryover.go`), never a hand-edit. Verified independently of this document by
`TestC1181_003_CarryoverEntryRetiredFromLiveState`, which re-reads the live state file, checks the
rest of the backlog survived, and requires `statemapRevision` to have strictly advanced (while `stateRevision` must remain at or above its pre-cycle baseline).

## Precedent this closes (read before writing the next resolution doc)

Cycle 1164 attempted this same retirement and **failed audit (D1 HIGH,
`.evolve/state.json:1013`)**: its shipped doc asserted a `carryoverTodos` removal that the live
state never received — the state stayed byte-identical at rev 1704 with the entry resident, so the
carryover re-fired. The lesson generalises: **a resolution doc is prose until the live state loses
the entry.** Every claim in this document is therefore backed by a predicate that reads production
state or drives production code, not by the document asserting it.
