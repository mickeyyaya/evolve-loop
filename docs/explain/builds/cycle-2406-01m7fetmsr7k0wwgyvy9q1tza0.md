# Build Explanation — Cycle 2406

## Build Binding
- Cycle: 2406
- Base SHA: 109b1b22f1c889be72db4ec0ff88ee21f2e520d6

## Summary
A CLI family that fails its boundary update smoke test now benches itself in `clihealth`. The loop continues when fallback families cover all routes.

## Rationale
An unconditional halt stops the entire loop when one CLI family breaks after an update. Benching the broken family lets healthy families run the wave.

## Changed Areas
- `go/cmd/evolve/cmd_loop_cliupdate.go` — checks routing chains and floors before it halts, benches bypassable smoke failures, and emits one incident per family.
- `go/internal/clihealth/clihealth.go` — adds `SmokeFailedPattern` (`smoke_failed`) to `Benchable` so smoke failures bench with a cooldown and evidence.
- `docs/architecture/packages/internal-clihealth.md` — records the `SmokeFailedPattern` contract and the fallback-check behavior.

## Design Decisions
The loop checks `cliroute.FamilyChains` and `Policy.Floors()` against active benches plus smoke-failed families. A missing router or an uncovered chain still halts the loop.

## Verification
All five cycle-2406 ACS predicates, `internal/clihealth` tests, `cmd/evolve` smoke and boundary tests, and `ste-lint --strict` pass after the final edit.

## Compatibility
Existing halt behavior stays intact when no routing table exists or when any required chain or floor loses its last runnable family.

## Limitations
A family benched by `smoke_failed` stays benched until its `clihealth` cooldown expires or an operator clears it.
