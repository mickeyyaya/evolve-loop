# Build Explanation — Cycle 2388

## Build Binding
- Cycle: 2388
- Base SHA: f86b7a1cb1606f4111548ab6a748ee56e36f4291

## Summary
`bridgechain.Walking.Launch` retries the primary CLI once in a fresh session when it fails with `submit_wedged` or `transient_upstream` and all fallback candidates are benched.

## Rationale
When a primary CLI fails with a transient cause and every fallback family is benched, a direct fallback wastes the cycle on an unhealthy family. Retrying the primary once with `FreshSession: true` gives the healthy primary a clean session before the walk moves on.

## Changed Areas
- `go/internal/bridgechain/bridgechain.go` — adds `launchWithTransientRetry`, `shouldRetryPrimaryTransient`, `isTransientLaunchCause`, `allRemainingBenched`, and `candidateIsBenched` to retry a transient primary failure once in a fresh session.
- `go/internal/bridgechain/fresh_session_test.go` — adds unit tests covering transient retry, empty project root, healthy fallback, boot-timeout bench, and model bench cases.
- `docs/architecture/packages/internal-bridgechain.md` — documents the transient primary retry rule in STE under `## Invariants`.

## Design Decisions
- The retry lives in `Walking.Launch` right after `core.LaunchWithFreshSession` rather than inside `llmroute.DispatchTiered`, because `bridgechain` owns `clihealth` bench evaluation.
- `allRemainingBenched` reuses `CLIHealthOn`, `clihealth.NewStore`, `scopeCoversSeat`, and `seatModel` so family, driver, scoped, and model benches match `ApplyCLIHealthBench`.
- Non-transient causes (such as `missing_binary` or `incomplete`) and plans with at least one healthy fallback continue to walk immediately in bench order.

## Verification
Targeted unit tests in `go/internal/bridgechain` and all 7 cycle 2388 ACS predicates pass. `evolve docs ste-lint --strict` reports zero findings on the updated package note.

## Compatibility
Existing `Walking.Launch` callers, `ChainAttempt` pass-through semantics, and `BenchOnEscalation` behavior remain unchanged.

## Limitations
The retry is limited to `submit_wedged` and `transient_upstream` on the first candidate (`plan.Candidates[0]`) when `ProjectRoot` is set and `CLIHealthOn` is enabled.
