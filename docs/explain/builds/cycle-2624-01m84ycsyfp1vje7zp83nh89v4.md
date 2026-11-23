# Build Explanation — Cycle 2624

## Build Binding
- Cycle: 2624
- Base SHA: f652fc150adc04db2dc58becf3162c7c8778d883

## Summary
`evolve ship --paths` now ships a deletion that is already staged. A path that no source knows still fails and names the path.

## Rationale
`git add -A -- <path>` exits 128 when the path is gone from disk and from the index. The ship step now checks each named path first and stages only the paths that git can match.

## Changed Areas
- `go/internal/phases/ship/paths.go` — adds `stageablePaths`, which keeps paths on disk or in the index, skips staged deletions, and refuses unknown paths by name. Adds `stageManualSet`, which calls `stageablePaths` and skips `git add` when no path remains.
- `go/internal/phases/ship/verify.go` — `verifyManualConfirm` calls `stageManualSet` in place of a direct `git add -A`.
- `go/internal/phases/ship/stage_manual_paths_deletion_test.go` — tests for a staged deletion, a mixed set, and an unknown path.

## Design Decisions
The check lives in the ship package beside the existing `--paths` helpers. No new flag or option exists.

## Verification
The integration tests `TestManualShipPaths_*` and the four cycle-2624 ACS predicates pass.

## Compatibility
Paths on disk and unstaged deletions stage as before.

## Limitations
Ten other integration tests of the ship package fail with and without this change.
