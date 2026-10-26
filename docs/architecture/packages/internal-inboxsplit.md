# internal/inboxsplit

> The decision: [ADR-0139](../adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md). The spec: [wave orchestrator §4](../wave-orchestrator.md). The plan row: O2c in [the plan](../../plans/wave-orchestrator-2026-10.md). The verb: `evolve inbox split` in [the runtime reference](../../operations/runtime-reference.md). The predicate: [internal-guards](internal-guards.md) and [internal-lanerouting](internal-lanerouting.md).

## Purpose

`internal/inboxsplit` splits one mixed inbox item into two items. A mixed item names protected files and files that are not protected. A census of 2026-10-17 found only 1 of 168 unrouted items safe for a lane. The split frees the lane part. The guard stays on the console part.

A lane part must be ready for a lane. These four rules define the term:

1. It holds no protected path.
2. Its text names no protected path and no console file.
3. It carries a criterion that a grader can judge.
4. It waits for the console part when it needs that part.

## Design

- **Pure proposal.** `Propose(item, isProtected, laneAfterConsole)` reads one item and returns a `Proposal`. It reads no file and never changes the item it reads.
- **The lane predicate and containment decide.** `Propose` takes the predicate of `laneForbidden` and `guards.TouchesProtectedSurface`. A path is protected when either says so. Containment adds a path that equals a fragment, lies inside one, or holds one. The claim floor and `evolve inbox route-lane` keep scope, so they accept some items that the split refuses. That trade-off keeps four lanes supplied.
- **A files entry is judged by its path tokens.** An entry can carry a note, such as `go/internal/core/ (ReviewInput)`. `Propose` reads the path tokens of the entry with `inboxbatch.Item.DeclaredPaths`, as the routing check does. The entry is protected when the whole entry or one token is protected.
- **The console part keeps every protected file.** `TestInboxSplit_TheConsolePartKeepsEveryProtectedFile` pins this. The part keeps the `route` when the route is a console route.
- **The lane part holds no protected file.** It has no `route`. `Accept` routes it with `RouteLane`, which applies the claim floor, so a lane part that must stay on the console is refused.
- **The acceptance is divided by the files that it names.** `TestInboxSplit_TheLanePartKeepsTheCriteriaThatNameOnlyLaneFiles` pins the rule. A criterion names a file when it holds a protected path, or a path or a base name from the `files` list. A slash in prose, such as `refuses/stamps`, names nothing (`TestInboxSplit_ASlashInProseDoesNotMakeACriterionNameAFile`).
  - A criterion that names a protected path, or the base name of a console file, stays on the console part.
  - A criterion that names only lane files goes to the lane part.
  - A criterion that names no file judges the whole item, so both parts keep it.
  - `Propose` writes no criterion of its own. A generated criterion such as "the change is made in these files only" passes on any edit, so a grader cannot judge intent with it.
- **A part with no criterion is refused.** `Propose` returns `ErrNoCriterion` and names the part. The inbox also refuses a filing with no criterion. The package cannot judge if a criterion probes behavior, so it checks only that one exists.
- **The text names no console file.** `Propose` reads `title`, `fix`, `summary`, `description`, `source` and `connects_to`. The lane part copies these fields. `Propose` returns `ErrFixNamesProtected`, with the name, when a field has a protected path token or the base name of a console file. A lane that follows such text edits a protected file and fails the integrity guard.
- **The lane part waits when it only verifies.** When every lane file is a ratchet file or a test file, `LaneWaits` is true and the `deps` name the console part. A ratchet file is `.cover-strict` or `.apicover-enforce`. A test file ends in `_test.go`. Such a part has nothing to verify until the console part lands. `--lane-after-console` forces the wait for any lane part.
- **The lane part carries only allowlisted fields.** `laneFields` lists them: the fields that the claim floor, the batcher and the dispatch read, and the text of the fix. The console part keeps the full item. A field that nobody listed, such as `notes` or a future field, never reaches a lane.
- **Each part keeps the origin.** `laneFields` lists `origin`, and the console part keeps every field. The split verb files the parts with the origin `console` when the original names none (2026-10-27, [internal-inboxcurator.md](internal-inboxcurator.md)).
- **No empty `deps`.** A lane part with no dependency has no `deps` key.
- **Ids and links.** The ids are `<id>-console` and `<id>-lane`. Both parts carry `split_from: <id>`.
- **Supply gate.** `Proposals(items, admissible, window, isProtected)` returns proposals only when `admissible` is below twice `window`. The census of O6 calls it.
- **Accept order.** `Accept` files the console part, files the lane part, routes the lane part and retires the original. It uses the `Inbox` port. A failure before the route withdraws the parts that it filed. A failure of the retire leaves both parts and returns `ErrRetireFailed`. `Result.LaneWaits` tells the command if the lane part waits.
- **No split when none is needed.** An item with only protected files returns `ErrAllProtected`. An item with only files that are not protected returns `ErrNoneProtected`. The console routes that item with `evolve inbox route-lane`.
- **Idempotence.** The original is no longer pending after a split, so a second `Accept` returns `ErrNotPending` and writes nothing.
- **Evidence.** `Accept` never withdraws the original. The port retires it, and the retired file keeps every field.

## Decisions and rejected options

The live review of 2026-10-18 ran 34 real splits and reverted all of them. It found five defects in the lane parts. Each row gives the choice and the options that were not chosen.

| Defect | Choice | Rejected options and why |
|---|---|---|
| A directory that holds a fragment went to a lane part | A containment rule, in `guards.TouchesProtectedSurface`, for the split only | Change `IsProtectedScope`: tests and ADR-0139 decision 5 pin it as membership. Apply the rule to every routing root: lane-routable items fall from 3 to 1, and the operator wants four lanes. |
| The lane criterion was a stub | Divide the real criteria by the files that they name | Keep the stub: it passes on any edit. Copy every criterion to both parts: a lane cannot meet a criterion about a console file. |
| The lane text named console files | Refuse the split | Derive a lane-scoped text: prose has no parser, so a derived text can drop a constraint. Add a scope line to the text: the lane can still follow the body. |
| The lane part needed the console part and had no `deps` | Add the console part to `deps` for a part that only verifies | Refuse the split: the package knows the answer, so the console need not choose a flag. Leave `deps` empty: the lane builds a registry line or a test with nothing to verify. |
| `evolve inbox route-lane` replaced an earlier reason | Keep the earlier reason in the new one | A new `prior_routed_reason` field: it needs a new field role and a new rule for each consumer. |

## Measures

On the tracked inbox of commit `c2d0edb6c` (227 items), a throwaway test ran the old splitter and the new one on every item. The old splitter used `IsProtectedScope`, as the live review did. The test was not committed.

| Measure | Before | After |
|---|---|---|
| Splits proposed | 112 | 73 |
| Lane parts that list a protected path or a directory that holds one | 17 | 0 |
| Splits whose lane text names a base name of a console file | 28 | 0 |
| Lane parts with a generated criterion | 112 | 0 |
| Lane parts that only verify and have no dependency on the console part | 3 | 0 |

The new splitter refuses 60 items for their text. It refuses 8 items for a lane part with no criterion: each of the 8 has no acceptance at all. It refuses 86 items for the earlier reasons. Of the 73 splits, 2 wait for the console part.

## Limits

- The package does not decide which part needs the other, except for a lane part that only verifies. The console chooses `--lane-after-console` for the other cases.
- The token rule is broad on purpose. It reads any `a/b` string as a path. A false match refuses a split, and the console can then edit the text.
- The base name rule reads a word with an extension, such as `ship.go`. A generic name can match a file in another directory. The split is then refused, which is the safe result.
- A criterion can name a file by a module-relative path, such as `internal/core/x.go`. That path has no `go/` prefix and is not in the `files` list. The predicate does not match it, so the criterion names nothing and both parts keep it.
- The package cannot tell if a criterion probes behavior. It refuses a part with no criterion. The console can add one with `evolve inbox edit`.
