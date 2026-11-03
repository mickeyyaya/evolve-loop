# Build Explanation — Cycle 2141

## Build Binding
- Cycle: 2141
- Base SHA: d2b25492effb2b45d42a9f51013104e19371a9cc

## Summary
The router digest now measures only top-level report artifacts instead of every file in the run directory.

## Rationale
Routine prompts, logs, and event streams exceeded the 100 KB threshold on 131 of 133 replayed cycles. Counting only reports restores the discriminative power of the trigger.

## Changed Areas
- `go/internal/router/digest.go` — measures only `*-report.md` and `handoff-*.json` artifacts in the workspace root.
- `go/internal/router/trigger_producers_test.go` — tests that `run_dir.artifact_bytes` sums report artifacts rather than noise.
- `.evolve/phases/context-condense/phase.json` — records replay empirical statistics and updates the trigger threshold to 51200 bytes.
- `docs/architecture/packages/internal-router.md` — documents that `ArtifactBytes` counts report artifacts and removes the filed limit.

## Design Decisions
The filter checks top-level regular files matching `-report.md` or `handoff-*.json`. Subdirectories and non-report files do not add to the sum.

## Verification
ACS predicates for cycle 2141 and targeted router unit tests pass with zero failures.

## Compatibility
Existing routing configurations that evaluate `run_dir.artifact_bytes` continue to function with more accurate byte counts.

## Limitations
This change counts only top-level report and handoff artifacts in the cycle workspace directory.
