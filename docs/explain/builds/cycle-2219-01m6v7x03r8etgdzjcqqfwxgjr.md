# Build Explanation — Cycle 2219

## Build Binding
- Cycle: 2219
- Base SHA: 3c31ffb961bfbbbfe8299bdb40733eb375b7ad90

## Summary
The gc command now computes live runs before the dry-run check so dry runs report active leases and skip process sweeps.

## Rationale
Previously, runGC checked the dry-run flag before it retrieved live leases. Dry runs skipped the live lease notification and previewed process terminations that real runs skip.

## Changed Areas
- `go/cmd/evolve/cmd_gc.go` — computes live run directories before the dry-run check so dry runs respect active run leases.

## Design Decisions
Storage of liveRuns in the plan structure before the dry-run check reuses the existing liveRunDirs helper without new flags.

## Verification
Targeted unit tests in cmd_gc_live_lease_test.go and five ACS predicates in cycle 2219 verify dry-run preview behavior with and without active leases.

## Compatibility
The command line arguments and JSON structure remain unchanged. Dry runs still acquire no locks and write no state.

## Limitations
The real run now takes its snapshot before it acquires the lease.
