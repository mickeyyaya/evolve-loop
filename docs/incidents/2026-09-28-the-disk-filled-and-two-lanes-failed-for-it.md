# The disk filled, and two lanes failed for it (wave 27, 2026-09-28)

## What happened

Wave 27 (cycles 1739 and 1740, both size-ratchet shrink lanes) went 0/2.

- **1739** passed its audit. Its ship then failed the repo-contract importer backstop: 15 or more `internal/core` tests failed in 0.00s, and one of them said `TempDir: mkdir …: no space left on device`.
- **1740** failed its audit on the integration-tier CI-parity gate with 11 offenders.

The host volume had 143 MiB free of 460 GiB. The operator stopped the loop by its exact pid (a system failure halts; ADR-0072) and freed the disk before resuming.

Nothing in the pipeline noticed the disk. Both lanes' failures read as code failures, and a full disk nearly reset a consecutive-ship streak of 3.

## Where the space went

| Consumer | Size | Cause |
|---|---|---|
| `~/Library/Caches/go-build` | 157 GB | Go trims only cache entries unused for 5 days. The loop builds continuously, and every build of a lane's worktree (race, cover, integration, per-lane `-ldflags` stamps) adds new entries. Nothing bounded it. |
| `$TMPDIR/acs-cycle1515-bin-*` (1,367 dirs) and `acs-cycle1498-bin-*` (60) | 32 GB | See the root cause below. |
| `$TMPDIR/go-build*` (600 dirs) | 18 GB | Go's own work dirs, left behind when a build is killed mid-run (timeouts, observer kills). |
| `$TMPDIR/cycle*`, `Test*`, `release-pipeline-dryrun-*` | about 4 GB | Fixtures of killed tests, and dry-run journals that are never removed. |

## Root cause of the 32 GB: a deferred cleanup that `os.Exit` skipped

`acs/regression/cycle1515` and `acs/cycle1498` build the evolve binary in `TestMain`:

```go
dir, _ := os.MkdirTemp("", "acs-cycle1515-bin-")
defer os.RemoveAll(dir)
…
os.Exit(m.Run())
```

`os.Exit` never runs deferred calls, so each run leaked a temp dir holding a roughly 22 MB `evolve-under-test` binary.

`acs/regression/…` is the durable ACS suite. It runs in every audit's CI-parity gate, in every console floor and in CI, which is why one predicate accounted for 1,367 leaked binaries.

Since Go 1.15, a `TestMain` that simply returns uses `m.Run()`'s result as the exit code, so no `os.Exit` is needed. Measured on this host, the unfixed predicate leaks one dir per run (1,367 → 1,368) and the fixed one leaks none.

## Fixes

1. **Built.** Both `TestMain`s now return instead of calling `os.Exit`.
   - `internal/testmainexit` finds any `TestMain` that defers a cleanup and also calls `os.Exit`. Closures are excluded, because their defers run when the closure returns.
   - `TestModuleTestMainsNeverDeferCleanupPastOsExit` runs it over every bound test file (`rawgitratchet.BoundTestFiles`). Red first, it named exactly these two files.
2. **Queued: `disk-space-preflight`.** The loop's preflight halts when free space is below a configured floor and names the fix. A full disk is a system failure and must never reach a lane as a code FAIL.
3. **Queued: `gc-pipeline-temp-and-go-cache`.** `evolve gc` reaps the pipeline's stale temp artifacts (`go-build*` from killed builds, leftover `acs-*` and fixture dirs past an age) and bounds the Go build cache at a configured size. The operator frees space through the CLI, not by hand.

The emergency cleanup used Go's own CLI (`go clean -cache`, 157 GB) with no loop running. The 57 GB of temp leftovers wait for fix 3, so they are reaped through the interface.
