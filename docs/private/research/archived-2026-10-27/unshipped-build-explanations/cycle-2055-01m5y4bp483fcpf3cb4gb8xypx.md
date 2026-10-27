# Build Explanation — Cycle 2055

## Build Binding
- Cycle: 2055
- Base SHA: 1d361aef266b33532296574243e4b77a0e7c44aa

## Summary
Each quota observation is now appended to `.evolve/usage-history.ndjson`, one line per window. The snapshot file `usage-windows.json` still holds only the latest observation.

## Rationale
The snapshot overwrites itself on each probe, so past quota use is lost. The record step already runs after each probe on both probe paths. One append inside that step covers both paths with no new caller.

## Changed Areas
- `go/internal/usageprobe/history.go` — adds `HistoryFile`, `HistoryPath` and `appendHistory`, which write one NDJSON line per window with the CLI name and the observation time.
- `go/internal/usageprobe/windows.go` — `RecordObservation` calls `appendHistory` after the atomic snapshot write, inside the same lock.

## Design Decisions
A history failure returns an error from `RecordObservation`. Both callers already warn on that error and continue, so the probe still acts. The snapshot is written first, so it holds the latest value even when the append fails. Rejected: a separate history writer on each probe path, because it duplicates the call and misses future paths.

## Verification
ACS predicates TestC2055_001 to TestC2055_006 pass. They cover two probes giving two lines, one line per window, kept earlier lines, the evidence path, a failed write, and unchanged `clihealth usage` output. The usageprobe package tests pass.

## Compatibility
The snapshot format and `evolve clihealth usage` output are unchanged. The new file is only created when an observation has windows.

## Limitations
The history file has no rotation or size limit. A reader must tolerate a partial last line only if the process dies during a write.
