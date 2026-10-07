# Build Explanation — Cycle 1822

## Build Binding
- Cycle: 1822
- Base SHA: 6292cf2bf35fe7fac01f83cd83185ff5c5de0911

## Summary
`RunPool` with a nil launcher now fails every lane with the same result shape `Supervisor.Run` uses (`Index` i, `ExitCode` -1, `errNoLaunch`), instead of returning zero-value results that read as `LaneOK`. The quota-bench WARN moves to the `[fleet] WARN:` prefix that the freshness gate and the plan-time gate already use. The bounded-concurrency supervisor test now waits on a channel with a timeout instead of spinning. The global-zone file list is pinned to its documented set.

## Rationale
A nil launcher means no lane ran. Reporting success for it is the silent no-op the package's "fail loud" invariant forbids, and `Supervisor.Run` already fails the same input. Sharing one result shape lets callers treat both entry points the same way. `[fleet] WARN:` was the prefix most fleet WARN lines already used (freshness gate, plan-time gate, the loopwave golden stderr files), so moving the two quota lines to it was the smallest change that gives one prefix. The test's empty `for` loop burned a CPU and could hang forever if a launch never arrived. A select with a timeout fails within five seconds instead.

## Changed Areas
- `go/internal/fleet/pool.go` — a nil `LaunchFn` returns `noLaunchResults(len(backlog))`, every entry `{Index: i, ExitCode: -1, Err: errNoLaunch}`. The empty-backlog early return was dropped because the dispatch loop already returns an empty slice for it. Pulling the fill into a helper keeps `RunPool` within its function-size ratchet allowance.
- `go/internal/fleet/quota.go` — both quota-bench WARN lines now start `[fleet] WARN: quota bench …` instead of `[loop] WARN: fleet: quota bench …`.
- `go/internal/fleet/fleet_test.go` — `TestSupervisor_BoundedConcurrency` gets a buffered `entered` channel and waits for two launches through `select` with a `time.After(5s)` case. This replaces the busy-wait loop and the unused `started` WaitGroup.
- `go/internal/fleet/pool_test.go` — `TestRunPool_NilLaunch_FailsEveryLaneWithErrNoLaunch` checks the per-lane failure shape and `LaneFailed` status, that no transitions are emitted, and that an empty backlog still gives no results.
- `go/internal/fleet/packagegraph_test.go` — `TestGlobalZoneFiles_MatchesDocumentedGlobalZone` parses the documented global-zone clause and requires `GlobalZoneFiles` to match it exactly.
- `go/internal/fleet/quota_test.go` — `TestFleetWarnings_QuotaAndFreshnessShareOneModulePrefix` requires that the quota and freshness WARNs share one prefix and that the prefix names the fleet module.
- `docs/architecture/packages/internal-fleet.md` — records that both launch entry points share the nil-launcher failure shape, names the shared WARN prefix, and names the global-zone pin test.

## Design Decisions
`errNoLaunch` stays the single sentinel. `RunPool` reuses it instead of defining a pool-specific error, so `errors.Is` checks work for both entry points. The prefix change goes toward `[fleet]` and not `[loop] … fleet:`, because the `[fleet]` form already appeared in two golden files and the plan-time gate, while inside `internal/fleet` the `[loop]` form appeared only in `quota.go`.

## Verification
- `go test -count=1 ./internal/fleet`: ok. `go test -count=1 ./internal/loopwave`: FAIL in `TestSize_BenchedFamiliesReadFromTheStore` (`go/internal/loopwave/plan_test.go:111`), which still expects the old `[loop] WARN: fleet: quota bench` text. That file is on the protected control plane, so this build cannot update it.
- `go test -tags acs -count=1 ./acs/cycle1822`: 4/4 predicates pass.
- `go test -count=1 -run '^TestRatchet_' ./cmd/evolve`: passes, so the function-size ratchet holds.
- Each eval grader's named test reports `--- PASS`.

## Compatibility
Anything matching the old `[loop] WARN: fleet: quota bench` text in stderr will no longer match; the only in-repo consumer is loopwave's `TestSize_BenchedFamiliesReadFromTheStore`. It sits under the protected `go/internal/loopwave/`, so this cycle leaves it unchanged, and it fails until the console updates its expected string to `[fleet] WARN: quota bench on CLI family "codex" (rate_limit): wave count 3 -> 2 (min 1)`. The production caller, `dispatchPoolIteration` in `go/cmd/evolve/cmd_loop_pool.go`, forwards the loop's launch seam to `RunPool`. With a non-nil launcher, behavior is unchanged. Only a nil one, which is a misconfiguration, now reports every lane as failed instead of as a success.

## Limitations
`[loop] WARN:` lines outside `internal/fleet` keep their prefix; this change unifies only the fleet package's own WARNs. The prefix change is incomplete until the console updates the expectation in `go/internal/loopwave/plan_test.go:111`. Moving the freshness line to `[loop] WARN: fleet:` would not avoid that work, because it would break loopwave's golden stderr files, which are just as protected.
