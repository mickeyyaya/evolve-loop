# internal/inboxsplit

> The decision: [ADR-0139](../adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md). The spec: [wave orchestrator §4](../wave-orchestrator.md). The plan row: O2c in [the plan](../../plans/wave-orchestrator-2026-10.md). The verb: `evolve inbox split` in [the runtime reference](../../operations/runtime-reference.md).

## Purpose

`internal/inboxsplit` splits one mixed inbox item into two items. A mixed item names protected files and files that are not protected. A census of 2026-10-17 found only 1 of 168 unrouted items safe for a lane. The split frees the lane part. The guard stays on the console part.

## Design

- **Pure proposal.** `Propose(item, isProtected, laneAfterConsole)` reads one item and returns a `Proposal`. It reads no file and never changes the item it reads.
- **The console part keeps every protected file.** `TestInboxSplit_TheConsolePartKeepsEveryProtectedFile` pins this. The part keeps the `route` when the route is a console route, and it keeps the acceptance.
- **The lane part holds no protected file.** It has no `route`. `Accept` routes it with `RouteLane`, which applies the claim floor, so a lane part that must stay on the console is refused.
- **No split when none is needed.** An item with only protected files returns `ErrAllProtected`. An item with only files that are not protected returns `ErrNoneProtected`. The console routes that item with `evolve inbox route-lane`.
- **The text names no protected path.** `Propose` reads the path-like tokens of `fix`, `acceptance`, `summary` and `description`. It returns `ErrFixNamesProtected`, with the path, when `isProtected` judges one of them protected. The lane part copies `fix`, so a fix that says "edit a protected file" sends a lane to work that it must not do.
- **The lane part carries only allowlisted fields.** `laneFields` lists them: the fields that the claim floor, the batcher and the dispatch read, and the text of the fix. The console part keeps the full item. A field that nobody listed, such as `notes` or a future field, never reaches a lane.
- **Ids and links.** The ids are `<id>-console` and `<id>-lane`. Both parts carry `split_from: <id>`.
- **Supply gate.** `Proposals(items, admissible, window, isProtected)` returns proposals only when `admissible` is below twice `window`. The census of O6 calls it.
- **Accept order.** `Accept` files the console part, files the lane part, routes the lane part and retires the original. It uses the `Inbox` port. A failure before the route withdraws the parts that it filed. A failure of the retire leaves both parts and returns `ErrRetireFailed`.
- **Idempotence.** The original is no longer pending after a split, so a second `Accept` returns `ErrNotPending` and writes nothing.
- **Evidence.** `Accept` never withdraws the original. The port retires it, and the retired file keeps every field.

## Limits

- The package does not decide which part needs the other. The console chooses `--lane-after-console`.
- The token rule is broad on purpose. It reads any `a/b` string as a path. A false match refuses a split, and the console can then edit the text.
- The lane part has one generated acceptance criterion: the change that the original describes is made in the lane files only. The console can edit it with `evolve inbox edit`.
