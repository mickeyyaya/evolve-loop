# internal/gittest

## Purpose

`internal/gittest` owns git-backed test fixtures: their creation and their teardown.

## Design

- Every repo the fixture builds persists a config that turns git's automatic maintenance off.
- From git 2.47 a commit or fetch detaches a `git maintenance run --auto` child that keeps writing into `.git/objects`. So `maintenance.auto=false` is set as well as `gc.auto=0`.
- `gc.auto=0` alone does not stop the detached child. It covers gits older than 2.47, whose commit runs `gc --auto` directly.
- The same config carries a test identity, so commits work on a runner with no global identity. Production code that runs a raw git inside a fixture inherits the maintenance guarantee.
- Teardown retries removal with a doubling backoff, bounded by `teardownAttempts`. The backoff waits about 1.3 seconds in total. Then the test fails.
- The failure names the processes that hold the tree. They come from `lsof` on `PATH`, looked up at failure time.
- `lsof` exits 1 even when it prints holders. So `holders` judges its output, not its status. No test pins this rule, because it needs a real holder.
- When `lsof` is absent, the failure text says "diagnostic unavailable". Pinned by `TestRemoveWithRetry_ExhaustionFailsWithPathAndDiagnostic`.

## Invariants

- Removal of a fixture tree is registered with `tb.Cleanup`, which runs LIFO. So the retried removal runs before the single `RemoveAll` of `TempDir`, which then finds nothing left to race. Pinned by `TestRemoveWithRetry_OutlastsATransientHold` and `TestRemoveWithRetry_ExhaustionFailsWithPathAndDiagnostic`.
- A fixture commits with no ambient git identity, and its own config keeps auto-maintenance off for any git run inside it. Pinned by `TestFixture_CommitsWithNoAmbientIdentity` and `TestMaintenanceConfig_TurnsBackgroundMaintenanceOff`.
- `Clone` persists the fixture config in its own repo, because production fetches inside a clone never pass through `Repo.Git`. A clone of a bare origin tracks its `main`. Pinned by `TestBareAndClone_CarryTheFixtureConfig`.

## Findings

- None open.
