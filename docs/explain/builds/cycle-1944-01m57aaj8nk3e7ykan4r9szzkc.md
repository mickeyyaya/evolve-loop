# Build Explanation — Cycle 1944

## Build Binding
- Cycle: 1944
- Base SHA: 20817ef7e1045cf2a8f88d878acfc066f3916cf6

## Summary
New behavior tests take four internal/core functions to 100.0% statement coverage. A new inbox item queues the rest of the package coverage work.

## Rationale
The inbox item core-package-coverage-batches-after-1942 asks for the whole package at 100.0%. One cycle cannot close it, so this batch closes four functions and queues the remainder. Production code stays unchanged, because each uncovered branch can be reached from a test.

## Changed Areas
- `go/internal/core/core_coverage_batch_lanescope_test.go` — adds TestCoreCoverageBatch_* tests for materializeLaneScope, scoutReportGoalHash, buildBudgetScale and SeedDispositionSkeleton, so that the batch predicate measures each at 100.0%.
- `.evolve/inbox/2026-10-18T12-00-00Z-core-package-coverage-batches-after-1944.json` — queues the rest of the package coverage and the go/.cover-strict registration, so that the unmet acceptance stays tracked.

## Design Decisions
Each error branch uses a real file system state: a regular file in the path, a directory at the target path, a missing workspace or a malformed ledger. No new seam or mock is added. The tests reuse writeBatchFile from the cycle-1938 batch file.

## Verification
The cycle predicates TestC1944_001 and TestC1944_002 pass. `go test -count=1 ./internal/core` passes.

## Compatibility
Production behavior does not change. The diff adds only a test file and an inbox item.

## Limitations
The package is not at 100.0% yet, so go/.cover-strict does not list ./internal/core.
