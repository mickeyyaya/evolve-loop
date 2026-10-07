# Build Explanation — Cycle 1823

## Build Binding
- Cycle: 1823
- Base SHA: cac6585f1a4c77c7c8f7b4a3e657809b32b77474

## Summary
`fleet.RunPool` no longer reports a nil launcher as a fleet of successful lanes: every backlog entry now fails with the same `Index i, ExitCode -1, errNoLaunch` shape `Supervisor.Run` returns. The dead nil-launcher branch in `Supervisor.launchOne` is gone, the global-zone file list is pinned to its documented set, and `TestSupervisor_BoundedConcurrency` waits on a channel with a timeout instead of spinning. This cycle continues the salvaged cycle-1822 worktree and fixes the three defects its audit found.

## Rationale
A nil `LaunchFn` returned `len(backlog)` zero-value results from `RunPool`, so every lane read as `LaneOK` while nothing ran. Returning the `Supervisor.Run` failure shape makes the two schedulers agree. The cycle-1822 audit rejected that build for three reasons. Its quota-bench WARN prefix change turned `internal/loopwave` red, because loopwave pins that text under the protected control plane. It also left the dead `launchOne` nil branch in place, and the package doc named a test that does not exist. This build restores the WARN text from the base, deletes the branch, and removes the false doc claim. Unifying the WARN prefix stays console follow-up F2.

## Changed Areas
- `go/internal/fleet/pool.go` — `RunPool` returns `noLaunchResults(len(backlog))` for a nil launcher, so each lane fails with `errNoLaunch` and no lane reads as a success.
- `go/internal/fleet/pool_test.go` — `TestRunPool_NilLaunch_FailsEveryLaneWithErrNoLaunch` checks the failure shape, that no transitions are emitted, and that an empty backlog returns no results.
- `go/internal/fleet/fleet.go` — deletes the unreachable `if s.Launch == nil` branch in `launchOne`. `Run` is its only caller, and `Run` rejects a nil launcher through `Validate` before scheduling any launch.
- `go/internal/fleet/fleet_test.go` — `TestSupervisor_BoundedConcurrency` now waits for two `entered` signals inside a `select` with a 5s `time.After`, replacing the busy-wait on `inFlight`.
- `go/internal/fleet/packagegraph_test.go` — `TestGlobalZoneFiles_MatchesDocumentedGlobalZone` holds `GlobalZoneFiles()` to the global-zone clause in the package doc.
- `docs/architecture/packages/internal-fleet.md` — documents the shared nil-launcher failure shape and the global-zone pin. It also drops the claim that the quota and freshness WARNs share a `[fleet] WARN:` prefix, a claim that was false and cited a missing test.
- `.evolve/evals/fleet-runpool-silent-success.md` — the task eval, retargeted to the `go/acs/cycle1823` predicates and extended with the WARN-prefix doc-truth criterion.
- `.evolve/evals/interaction-rollup-tmp-collision.md` — an eval for a separate inbox task that the harness had already staged in this worktree. Build carries it unchanged.
- `go/acs/cycle1823/predicates_test.go` — the TDD-owned acceptance predicates for this cycle (001-007), committed unchanged.
- `docs/private/research/archived-2026-10-07/superseded-predicate-packages/cycle1822/predicates_test.go` — the harness archived the cycle-1822 predicate package here when cycle 1823 took over the contract.
- `docs/private/research/archived-2026-10-07/unshipped-build-explanations/cycle-1822-01m4aashs7rpra1rvmn7n0xzyv.md` — the harness archived the unshipped cycle-1822 Build explanation here.

## Design Decisions
`RunPool` reuses `errNoLaunch` and the `Supervisor.Run` result shape rather than defining a new error, so callers classify both schedulers' failures the same way. The quota WARN text keeps its `[loop] WARN: fleet:` prefix because loopwave's pinned goldens sit under the protected control plane, which a fleet lane may not edit. The nil-launcher check now lives only in `Validate`, so the rule exists in one place.

## Verification
`go test -count=1 ./internal/fleet ./internal/loopwave` passes. `go test -tags acs -count=1 ./acs/cycle1823` reports 7/7 predicates PASS, including the loopwave-green, dead-branch and doc-truth checks. `gofmt -l` and `go vet` are clean for `internal/fleet` and `internal/loopwave`.

## Compatibility
No exported API changes. The only behavioral change is the `RunPool` result for a nil launcher, which used to be a silent success. Every WARN line is byte-identical to the base.

## Limitations
The quota WARN (`[loop] WARN: fleet:`) and the freshness WARN (`[fleet] WARN:`) still use different prefixes. Unifying them requires a console edit to loopwave's protected goldens (follow-up F2, `docs/architecture/decomposition/13-loopwave.md`).
