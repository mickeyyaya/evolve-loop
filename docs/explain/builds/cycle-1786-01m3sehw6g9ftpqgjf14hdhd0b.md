# Build Explanation — Cycle 1786

## Build Binding
- Cycle: 1786
- Base SHA: 313ce274b139964e050c15f3761a1fbc0b4a6b47

## Summary
`evolve loop-stop` gains `--wait [--timeout D]`, which blocks until no run lease is live and prints each live run and its phase when they change. A new read-only `evolve status [--json]` reports the loop, the cycles, the consecutive ship streak with the last zero-ship run, open PRs with their check state, and the failing jobs of main's latest required CI run. `evolve sync-main` and `loop-stop --wait` now read liveness through one shared function.

## Rationale
An operator needs to know when the brake has actually drained the loop, and needs one command that answers the Status Reporting checklist: ship streak, open PRs, failing CI job names. Every answer reuses an existing source of truth: the run lease for liveness, the dashboard collector for loop and dossier state, and ciwatch's gh fetcher for CI.

## Changed Areas
- `go/internal/runlease/runlease.go` — adds `LiveOwner` (one run dir: lease present, heartbeat within `DefaultTTL`, owner pid alive via `PIDAlive`) and `LiveRuns`, which scans a runs directory through `LiveOwner`; one verdict for every caller.
- `go/internal/runlease/runlease_test.go` — proves both skip stale, dead-pid and lease-less runs against a real live and a real exited pid.
- `go/cmd/evolve/cmd_syncmain.go` — replaces its inline `Read` + `OwnerLive` with the cmd-local `pidAlive` by `runlease.LiveOwner`, so it cannot drift from `loop-stop --wait` (a pid owned by another user now reads live to both).
- `go/cmd/evolve/cmd_loop_stop.go` — adds `--wait`/`--timeout`, rejects `--wait --release` with exit 10 and `--timeout` without `--wait`, polls `LiveRuns` after engaging the brake, and prints a live run's id and `run.json` phase only when that pair changes.
- `go/internal/dashboard/model.go` — adds `Trend.ShipStreak`, `Trend.LastZeroShipRun` and the `ZeroShipRun` type.
- `go/internal/dashboard/history.go` — computes the streak and last zero-ship run in `computeTrend` over the full history, before the 120-point cap, with the existing `shipped` predicate.
- `go/internal/dashboard/history_test.go` — table test for the streak, including a history longer than the cap.
- `go/internal/dashboard/apicover_named_test.go` — names `ZeroShipRun`.
- `go/internal/ciwatch/ciwatch.go` — adds `RunStatus.FailingJobs`.
- `go/internal/ciwatch/ghfetcher.go` — factors the run-list query into `latestRequiredRun`, shared by `NewGHFetcher` (`--commit`) and the new `LatestRequiredRunOnBranch` (`--branch`), and reads failing job names from the first tab-separated column of `gh run view --log-failed`, which the fetcher already requested.
- `go/internal/ciwatch/ciwatch_test.go` — covers the branch filter, the job-name extraction and error propagation.
- `go/cmd/evolve/cmd_status.go` — the `status` command: streak from the dashboard trend, open PRs summarised to `checks` failing/pending/passing/none plus `failing_checks`, CI via `ciwatch.LatestRequiredRunOnBranch(main)`, exit 2 when `.evolve/` is missing or not a directory.
- `go/cmd/evolve/cmd_status_test.go` — covers the check-state classification, the unreadable-snapshot check, and the phase-change output of the wait.
- `go/cmd/evolve/registry.go` — registers `status`.
- `go/cmd/evolve/main.go` — lists `status` and the new `loop-stop` flags in help.
- `docs/operations/runtime-reference.md` — documents both commands, and the wave-boundary Stop step now runs `evolve loop-stop --wait` and treats its exit 0 as the confirmation to merge.
- `docs/architecture/packages/internal-dashboard.md` — records the streak fields and why they are computed before the cap.

## Design Decisions
Liveness lives in `runlease` with the TTL and pid probe fixed inside `LiveOwner`, so callers cannot pass different parameters. The streak lives in the dashboard next to the `shipped` predicate, so the dashboard and `status` cannot disagree on what a ship is. CI goes through ciwatch's fetcher rather than a second gh query, filtered to `required.yml` on `main`; job names come from the `--log-failed` output the fetcher already pulls, so no gh call was added to the post-push watcher. PR check state is summarised rather than passed through raw, so the JSON stays stable across gh's CheckRun and StatusContext shapes.

## Verification
ACS `TestC1786_001`–`023` pass. `go test -count=1` passes for `internal/runlease`, `internal/dashboard`, `internal/ciwatch` and `cmd/evolve`.

## Compatibility
Plain `loop-stop` and `loop-stop --release` behave as before. The dashboard's trend JSON gains two fields. `NewGHFetcher` sends the same gh calls as before. `sync-main` now treats a lease owned by another user's live process as live, where it used to proceed.

## Limitations
PR and CI sections need an authenticated `gh`; without it they read `available:false`. "main" is the fixed branch for the CI section. Failing job names come from failed-job logs, so a job that failed without producing a log is not named.
