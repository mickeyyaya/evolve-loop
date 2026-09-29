# Build Explanation — Cycle 1766

## Build Binding
- Cycle: 1766
- Base SHA: f2a91dba9ddb2db6d4810acc8f5ba081cbf79cd6

## Summary
The flaking fence test's failure message now names the fence's `TakeErr`, `RestoreErr` and `Verified`. With that message, the flake reproduced locally (1 in 50 under `-race`) showed `TakeErr = <nil> RestoreErr = <nil> Verified = true`. So neither fence error caused the empty `kept`. The cause was a git racy-clean miss in the fence's throwaway index. `treefence.writeTreeMode` copied `.git/index` with a fresh mtime. Git then trusted a stale stat for a same-size file rewritten in place, within the timestamp tick the real index was written in. The copy now keeps the real index's mtime.

## Rationale
Git rehashes an index entry whose mtime is not older than the index file itself ("racily clean"). A throwaway index copy stamped "now" loses that protection. The conflict fixture's `lane line\n` and `peer line\n` are both 10 bytes, and the debugger rewrites `shared.md` in place (same inode) right after the orchestrator's rebase handling writes the index. That is the exact same-tick, same-size case, so `git add -A` on the copy skipped the file, the post-dispatch tree equalled the snapshot, and `Kept` came back empty with `Verified = true`. Preserving the mtime is a one-call fix that keeps the existing real-index seed, which exists for speed and for the declared-files mode. The rejected alternative was dropping the seed and rehashing every file: correct, but slower on every fence, and it changes the declared-files contract.

## Changed Areas
- `go/internal/treefence/fence.go` — `writeTreeMode` seeds its throwaway index through the new unexported `copyIndex`, which copies the real index and sets the copy's mtime to the real index's mtime, so git's racy-clean check behaves as it does on the real index.
- `go/internal/treefence/fence_test.go` — `TestFenceEnd_KeepsASameSizeRewriteInTheRealIndexsTimestampTick` pins the root cause deterministically: with `core.trustctime=false`, a writable file and the real index share one mtime, and a same-size in-place rewrite at that mtime must be kept (it failed with `Kept = []` before the fix). `TestFenceEnd_DistinguishesInertTakeErrFromGenuineEmptyRestore` covers `Begin`/`End` telling an inert fence (`TakeErr`) from a verified empty restore.
- `go/internal/core/ship_recovery_debugger_test.go` — `fencedDebugger` keeps the whole `treefence.Outcome`. The flaking test's `kept` assertion prints `TakeErr`, `RestoreErr` and `Verified`, so any recurrence names its cause. `initConflictRebaseRepoT` applies `gittest.MaintenanceConfig()` (fixture hygiene). New tests re-run the flaking test in a child process under a `git` shim that forces the fence's Take or Restore to fail, and check that the child's own failure message names `TakeErr` or `RestoreErr` with the error text.
- `.evolve/evals/fence-kept-empty-macos-ci-flake.md` — the task's eval and graders, written upstream this cycle.

## Design Decisions
The fix stays inside `treefence`, the one place that builds throwaway indexes, so every fence caller (audit, debugger, declared-files snapshots) gets it. `copyIndex` is unexported and has one caller. The test sets `core.trustctime=false` because each write moves ctime, and the test cannot hold ctime still. Only mtime, size and inode can match, as on a real same-second rewrite on CI.

## Verification
Before the fix, the new treefence test failed (`Kept = [], want [shared.md]`) and passes after it. Before the fix, the flaking core test failed 1 in 50 under `-race` with `TakeErr = <nil> RestoreErr = <nil> Verified = true`; after it, 100/100 pass under `-race`. The child-process graders pass for both the Take and Restore failure cases. The audit's inert-fence probe now prints the `TakeErr` text in the test's own failure line. The full `./internal/core/...`, `./internal/treefence/...` and `./internal/gittest/...` suites pass.

## Compatibility
No public API, CLI flag or config changed. `treefence` snapshots now rehash racily-clean entries the way git does on the real index. The trees they produce are correct where they used to be stale, and the extra cost is a rehash of files touched within the index's own timestamp tick.

## Limitations
The CI occurrences (train #721, #725) were not re-run here. Attributing them to this mechanism rests on the local reproduction matching their signature: an empty `kept` with no fence error. `gittest.MaintenanceConfig` in the fixture is hygiene, not the fix: the reproduced failure had no Take error, which rules out background maintenance breaking the fence's Take. Other callers that copy a git index elsewhere in the repo were not swept.
