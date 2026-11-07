# Build Explanation — Cycle 2208

## Build Binding
- Cycle: 2208
- Base SHA: 577f1b88708154769bdcf3231bd2ae8b194f15a9

## Summary
The sync-main command now refuses to merge while any run directory under the runs folder holds a live lease. Before, it checked only the lease named by the root cycle state file.

## Rationale
Fleet lanes keep their lease under the runs folder and write no root cycle state file. The old guard therefore found no lease, and a merge can change the tree under a live lane. The runlease package already scans every run directory for a live lease, so the fix reuses that scan.

## Changed Areas
- `go/cmd/evolve/cmd_syncmain.go` — a new helper checks the classic workspace lease first, then every run directory, and the command refuses on either.
- `go/cmd/evolve/cmd_syncmain_livelanes_test.go` — tests cover a live lane lease, a mix of stale and live leases, and a merge with no live run.
- `go/acs/cycle2208/predicates_test.go` — acceptance predicates for the three criteria of the task.
- `.evolve/evals/sync-main-refuses-live-lanes.md` — the eval that maps each criterion to its predicate.

## Design Decisions
The classic root lease check stays, because its workspace can sit outside the runs folder. The run scan is added beside it and no new setting is added.

## Verification
The three predicates TestC2208_001, TestC2208_002 and TestC2208_003 pass. The ACS suite reports red=0.

## Compatibility
The refusal message and the exit code are unchanged. A tree with no live lease still merges.

## Limitations
The check reads lease files once and does not hold a lock. A lane that starts after the check can still start during the merge.
