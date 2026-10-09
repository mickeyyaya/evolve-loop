# Build Explanation — Cycle 1841

## Build Binding
- Cycle: 1841
- Base SHA: 9b47ca6aeca2f51f9a5ead2652c470cf04bfb3c8

## Summary
Refuse route-console and route-lane CLI verbs mid-wave while an autonomous loop lane holds an active run lease, preventing concurrent inbox file mutations.

## Rationale
Routing verbs rewrite tracked inbox files in-place. If executed while an autonomous loop lane holds an active run lease, tracked repository state is mutated mid-wave, triggering audit tree-state verification failures. Checking the existing refusedMidWave helper brings routing verbs into parity with edit, withdraw, and verify.

## Changed Areas
- `go/cmd/evolve/cmd_inbox_route_console.go` — invokes refusedMidWave after argument validation to refuse execution and preserve inbox files when a live lane lease exists.
- `go/cmd/evolve/cmd_inbox_route_lane.go` — invokes refusedMidWave after argument validation to refuse execution and preserve inbox files when a live lane lease exists.

## Design Decisions
Reuses the shared refusedMidWave helper rather than introducing a separate lease checking mechanism. Validates command usage first so invalid arguments yield exit code 10 before lease checks.

## Verification
Verified by ACS predicates and unit tests asserting mid-wave refusal exits 1 naming the live lane, leaves target files byte-identical, allows routing when leases are stale, and preserves usage error priority.

## Compatibility
CLI flags and argument schemas remain unchanged; only behavior during active mid-wave runs is guarded.

## Limitations
Applies only to local run leases under .evolve/runs; does not coordinate across remote nodes.
