# Build Explanation — Cycle 2250

## Build Binding
- Cycle: 2250
- Base SHA: b25471c0ccaa9d1fbcc616a8a8777931e81658b1

## Summary
Decision D6 and component X5 of `docs/plans/gc-closes-the-legacy-gap-2026-11.md` are built. A lane has ended when it is not live, not paused, not bound to a continuation, and older than `salvage_after_hours`. A close-out dossier is not needed. gc now saves the uncommitted state of an ended lane and removes the tree. The branch stays.

## Rationale
The dry run of 2026-11-09 released 0 of 261 worktrees. A lane with no run directory had no dossier, so the old rule never called it finished. The age of the tree is the proof of the end. The two keep rules protect a quota-paused lane and a continuation lane, which hold no lease.

## Changed Areas
- `go/internal/gc/worktrees.go` — `PlanWorktrees` reads the paused lanes and the continuation registry once. `keptOrSalvaged` and the new `salvageReason` choose salvage-remove for an ended lane. A close-out dossier older than the salvage age still counts.
- `go/internal/gc/worktrees_ended_test.go` — tests for `laneHolds`, `holds` and `salvageReason`, and for the plan failure on an unreadable source.
- `go/internal/gc/worktrees_scan_backfill_test.go` — passes the new `laneHolds` argument to `scanWorktrees`.
- `go/internal/gc/worktrees_salvage_test.go` — the two trees that must stay are now young. The old fixtures used 48 hours and relied on the dossier rule.
- `go/internal/gc/worktrees_fuzz_test.go` — the property allows a salvage-remove for an unclosed tree that is past the salvage age.

## Design Decisions
`core.PausedLaneScopes` and `continuation.ListRegistryEntries` are the two sources of the keep rules, so gc has no copy of either. The registry binds a lane through its branch. A read failure of either source fails the plan, so a lane is never released on a guess. `salvage_after_hours` of zero still means never.

## Verification
`go test -count=1 ./internal/gc` passes, and each changed function has 100% coverage. The ACS predicates `TestC2250_001` to `TestC2250_007` pass.

## Compatibility
A tree with a close-out dossier behaves as before. The plan output keeps its shape. A corrupt continuation registry now stops `evolve gc` for worktrees, with a clear error.

## Limitations
The age of a lane is the modification time of its directory. A lane that a tool touches often looks young and stays.
