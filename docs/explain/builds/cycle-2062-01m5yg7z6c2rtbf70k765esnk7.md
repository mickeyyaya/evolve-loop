# Build Explanation — Cycle 2062

## Build Binding
- Cycle: 2062
- Base SHA: f8269cb929be8f9053d677959f8dc2d4d57e981e

## Summary
Each quota observation now adds one line per window to `.evolve/usage-history.ndjson`. The file `usage-windows.json` still holds only the latest observation.

## Rationale
A reader needs the past readings to see how a quota window fills. The snapshot file replaces its data at each probe. An append-only file keeps the data without a second writer path.

## Changed Areas
- `go/internal/usageprobe/history.go` — adds `HistoryPath` and `appendHistory`. Each window becomes one JSON line with `cli` and `observed_at`.
- `go/internal/usageprobe/windows.go` — `RecordObservation` calls `appendHistory` after the snapshot write, inside the same file lock.
- `go/internal/usageprobe/history_test.go` — tests the path, the line content, and the encode, open and snapshot error paths.
- `go/acs/cycle2062/predicates_test.go` — the seven ACS predicates of the task.
- `.evolve/evals/usage-observation-history.md` — eval graders for the task.
- `docs/private/research/archived-2026-10-27/superseded-predicate-packages/cycle2055/predicates_test.go` — archived predicates of an unshipped cycle.
- `docs/private/research/archived-2026-10-27/unshipped-build-explanations/cycle-2055-01m5y4bp483fcpf3cb4gb8xypx.md` — archived explanation of an unshipped cycle.

## Design Decisions
Both production callers share `RecordObservation`, so the CLI probe and the evidence query path write history from one place. A failed history write returns an error that the callers warn about, and the probe still acts. A helper `wrapHistory` adds the error context in one place. `clihealth usage` does not call the writer, so its output does not change.

## Verification
The `internal/usageprobe` tests pass with 100% statement coverage of `appendHistory`, `wrapHistory` and `RecordObservation`. The ACS suite for cycle 2062 reports no red predicate.

## Compatibility
The snapshot file format does not change. The history file is new, and a reader that ignores it sees no difference.

## Limitations
The history file has no rotation. A later task can add a size limit.
