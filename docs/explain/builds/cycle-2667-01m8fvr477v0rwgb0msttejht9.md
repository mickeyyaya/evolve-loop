# Build Explanation — Cycle 2667

## Build Binding
- Cycle: 2667
- Base SHA: 59e7b0750b9210903d273be167b7434d045f8691

## Summary
The dashboard queue view now counts items in `delivered/`, `rejected/`, and `quarantine/` beside the four prior states. `inboxmover.LifecycleStates` now serves as the single directory-backed state list for both `readQueue` and `fingerprint`. The dashboard web view also renders the new state counts.

## Rationale
`readQueue` and `fingerprint` previously read a local `lifecycleDirs` slice that omitted `delivered`, `rejected`, and `quarantine`. Items in `delivered/` stayed invisible in the queue summary. Moving the directory-backed state list into `inboxmover.LifecycleStates` removes that duplicate list and lets tests detect any uncounted state.

## Changed Areas
- `go/internal/inboxmover/dispatchstate.go` — adds `LifecycleStates` to return a fresh slice of the seven directory-backed states (`processing`, `processed`, `consumed`, `retry`, `delivered`, `rejected`, `quarantine`).
- `go/internal/inboxmover/dispatchstate_test.go` — adds `TestLifecycleStates_ListsDirectoryBackedStatesAsFreshCopy` to verify all seven states, exclude `pending` and `unknown`, and verify copy isolation.
- `go/internal/inboxmover/apicover_named_test.go` — adds `TestLifecycleStates_NamedExport` so `apicover` exercises the new export with an assertion.
- `go/internal/dashboard/model.go` — adds `Delivered`, `Rejected`, and `Quarantine` integer fields to `QueueSummary` with matching JSON tags.
- `go/internal/dashboard/queue.go` — removes `lifecycleDirs` and populates `QueueSummary` counts from `inboxmover.LifecycleStates` with `countJSON`.
- `go/internal/dashboard/sse.go` — updates `fingerprint` to stat each state directory from `inboxmover.LifecycleStates` instead of `lifecycleDirs`.
- `go/internal/dashboard/static/app.js` — updates `renderQueue` to display `delivered`, `rejected`, and `quarantine` counts in the inbox header.
- `go/internal/dashboard/queue_test.go` — extends `TestReadQueue_PendingInRankOrderAndLifecycleCounts` for the three new states and adds `TestReadQueue_EveryLifecycleStateHasACount` and `TestFingerprint_TracksDeliveredDirectory`.
- `go/internal/dashboard/apicover_named_test.go` — extends `TestAPICoverNamedExports` to reference `QueueSummary` fields `Delivered`, `Rejected`, and `Quarantine`.
- `docs/architecture/packages/internal-dashboard.md` — documents `inboxmover.LifecycleStates` as the source of queue counts and directory stamps.
- `docs/architecture/packages/internal-inboxmover.md` — documents `LifecycleStates` and its copy-safe contract.

## Design Decisions
`inboxmover.LifecycleStates` returns a fresh slice literal on every call instead of a shared package variable so caller mutations never leak. `readQueue` maps each state constant to its `QueueSummary` field pointer and skips nil entries, while `TestReadQueue_EveryLifecycleStateHasACount` fails loudly if a state lacks a JSON count. Reusing `inboxmover.LifecycleStates` in `sse.go` keeps change detection aligned with the queue counts.

## Verification
`gofmt -l`, `go vet`, and `evolve docs ste-lint --strict` pass with zero findings. Unit tests in `go/internal/dashboard` and `go/internal/inboxmover` pass with 100.0% statement coverage on `readQueue`, `countJSON`, `inboxDir`, and `LifecycleStates`. All six cycle 2667 ACS predicates in `go/acs/cycle2667` (`TestC2667_001` through `TestC2667_006`) pass (`6/6 PASS`).

## Compatibility
`QueueSummary` adds `delivered`, `rejected`, and `quarantine` integer fields to its JSON object and keeps all prior JSON keys unchanged. `inboxmover.LifecycleStates` is a new exported function with no change to existing `inboxmover` APIs or `retirementStates`.

## Limitations
`QueueSummary` uses fixed struct fields for each state in `LifecycleStates`, so a future state added to `inboxmover` still requires a matching field on `QueueSummary`. `TestReadQueue_EveryLifecycleStateHasACount` catches that omission at test time.
