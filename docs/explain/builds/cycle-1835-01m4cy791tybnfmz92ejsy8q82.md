# Build Explanation — Cycle 1835

## Build Binding
- Cycle: 1835
- Base SHA: c021d852988d3fc86776ed35225a0c456aa7cf93

## Summary
Restricts dossier orphan pairing strictly to `knowledge-base/cycles` by directory and cycle, ignores tracked modified pairs, adds a nil check in `Write`, and removes the dead `BuildOpts.LedgerPath` field.

## Rationale
Prevents accidental pairing across disparate directories and ensures tracked modified dossier files are never swept or recommitted as orphans. Hardens `Write` against nil dereference panics, and removes dead configuration fields from `BuildOpts` to eliminate API ambiguity.

## Changed Areas
- `go/internal/dossier/build.go` — removes the unused struct field `BuildOpts.LedgerPath` and its obsolete comment.
- `go/internal/dossier/sweep.go` — restricts orphan candidate pairs strictly to `knowledge-base/cycles`, groups candidates by directory and cycle, and skips tracked modified files.
- `go/internal/dossier/write.go` — guards against nil dossier input to return an informative error instead of panicking.

## Design Decisions
1. In `go/internal/dossier/sweep.go`, `groupOrphanPairs` filters candidate paths so that only those in `knowledge-base/cycles` matching `cycle-<N>.(json|md)` are considered, grouping candidates strictly by directory and cycle number.
2. In `go/internal/dossier/sweep.go`, `anyTracked` queries git tracking status via `git ls-files` to skip pairs that are already tracked modifications rather than untracked orphan pairs.
3. In `go/internal/dossier/write.go`, checking `if d == nil` immediately returns an error rather than dereferencing `d.Cycle`, preventing runtime panics for invalid caller arguments.
4. In `go/internal/dossier/build.go`, `LedgerPath` was completely unused by all callers and callers' options, so dropping it cleans up `BuildOpts` without altering runtime behavior.

## Verification
- Verified against all 5 ACS predicates in `go/acs/cycle1835/predicates_test.go`:
  - `TestC1835_001_SameCycleFilesInDifferentDirectoriesAreNeverOnePair`
  - `TestC1835_002_OnlyTheDossierDirectoryIsSwept`
  - `TestC1835_003_TrackedModifiedPairIsNotRecommitted`
  - `TestC1835_004_BuildOptsHasNoDeadLedgerPath`
  - `TestC1835_005_WriteNilDossierReturnsErrorWithoutPanicking`
- Verified existing package tests in `go/internal/dossier/` (`sweep_test.go`, `sweep_result_test.go`, `anchored_behavior_test.go`, `amplify_test.go`, `apicover_named_test.go`) pass with fixture paths placed under `knowledge-base/cycles`.

## Compatibility
Fully backward-compatible. `BuildOpts.LedgerPath` had no live callers. `SweepOrphans` contract remains unchanged while tightening candidate filtering to only the canonical dossier location.

## Limitations
Only orphan pairs located directly in `knowledge-base/cycles` are swept. Orphan pairs in other directories or tracked modified pairs are deliberately ignored.
