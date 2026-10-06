# Build Explanation — Cycle 1809

## Build Binding
- Cycle: 1809
- Base SHA: 493ff848fda83021dca736b49d91e1afffa83743

## Summary
This change fixes four defects in `go/internal/inboxbatch`:
- Inbox fields are truncated on a rune boundary, so a loaded field is never invalid UTF-8.
- A duplicated id resolves to the same earliest-filed record in connects, unified-commitment validation and plan-time routing.
- The file-area and operator-state rules read the files[] tokens that routing declares.
- A console-routed record that needed sanitizing carries a coded `INBOX_LOAD_WARNING` in the reason the plan-time gate prints.

The work resumes the cycle-1808 salvage and completes the parts that salvage left open.

## Rationale
Each defect let two readers of one inbox record reach different answers:
- A byte cut produced invalid UTF-8.
- Unstable sorting and per-reader maps let "first" and "last" holders differ.
- A second, looser path tokenizer disagreed with routing's `declaredTokens`.
- A load warning about an operator-owned record never reached the gate.

The fix is to have one source per question, shared by every reader. `go/internal/loopwave/dispatch.go` is protected and stays unchanged: it already prints the resolver's reason verbatim, so enriching that reason inside inboxbatch reaches the gate.

## Changed Areas
- `go/internal/inboxbatch/item.go` — `sanitizeItem` backs the cut off to a `utf8.RuneStart` boundary, so an over-limit value keeps the longest whole-rune prefix (the salvage change).
- `go/internal/inboxbatch/dir_scan.go` — `ScanDir` sorts by id with `sort.SliceStable`, so duplicate ids keep the directory's filing order and first-wins means earliest-filed. `LoadWarning` gains an unexported `file` link to the record it describes. The duplicate-id warning now says how references resolve, replacing "resolve ambiguously".
- `go/internal/inboxbatch/rules.go` — `indexByID` is first-wins and is the one shared id index. `fileAreaRule` reads `Item.DeclaredPaths()`. The salvage's second tokenizer, `pathTokens` and `locationSuffix`, is deleted.
- `go/internal/inboxbatch/unified.go` — `UnifiedCommitment.Validate` resolves members through `indexByID`, so it judges the same holder as connects and routing (the salvage change).
- `go/internal/inboxbatch/archetype.go` — `IsOperatorState` tokenizes each files[] entry with `declaredTokens`, the routing tokenizer. An entry that declares nothing still fails toward the normal pipeline.
- `go/internal/inboxbatch/consoleroute.go` — `RoutedResolver` scans once, resolves through `indexByID`, and for a routed item appends `; INBOX_LOAD_WARNING: <warning>` for each of that record's load warnings. Routing decisions and fail-open behavior are unchanged.
- `go/acs/cycle1809/predicates_test.go` — the TDD-authored cycle predicates, 13 tests covering all four defects.
- `.evolve/evals/inboxbatch-utf8-and-resolution.md` — the eval graders now point at `./acs/cycle1809`.
- `docs/architecture/packages/internal-inboxbatch.md` — the package page now states first-wins resolution, the coded load warning and the shared tokenizer, replacing the stale "keeps the last item" and "every file is under `.evolve/`" wording.
- `docs/private/research/archived-2026-10-06/superseded-predicate-packages/cycle1804/predicates_test.go` — archives the failed cycle-1804 predicates, which required a protected edit.
- `docs/private/research/archived-2026-10-06/superseded-predicate-packages/cycle1808/predicates_test.go` — archives the failed cycle-1808 predicates, superseded by cycle1809.
- `docs/private/research/archived-2026-10-06/unshipped-build-explanations/cycle-1804-01m47mf94ecsjtysnp7afp8hbm.md` — archives the unshipped cycle-1804 explanation.
- `docs/private/research/archived-2026-10-06/unshipped-build-explanations/cycle-1808-01m489b683wyy5nyapbg0aaeh1.md` — archives the unshipped cycle-1808 explanation, which cited a reverted protected hunk.

## Design Decisions
- The warning-to-record link is an unexported `LoadWarning.file` field, not a parse of the warning text. This keeps the exported surface unchanged, and a later rewording of a warning cannot silently drop the code.
- Only routed items get the code. A lane-routed item's reason stays empty, so the gate's output and every routing decision are unchanged.
- `fileAreaRule` flattens `DeclaredPaths()` per item. `IsOperatorState` tokenizes per entry, so that an entry declaring nothing still answers false.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1809` passes (13/13).
- `go test -count=1 ./internal/inboxbatch/ ./internal/loopwave/ ./internal/fleet/ ./internal/acssuite/` passes.
- The full module suite and the build handoff floor run before handoff.

## Compatibility
No exported identifier changes. Reasons for routed items with load warnings gain a suffix. No caller parses the reason: it is printed by `loopwave` and recorded by `fleet`. Backtick- or angle-wrapped files[] entries no longer bind by file area, because routing never declared them.

## Limitations
A record whose JSON does not parse has no readable route. It stays dispatchable, and its warning is not printed at the gate. Printing it there needs the protected `go/internal/loopwave/dispatch.go`, which this lane does not touch.
