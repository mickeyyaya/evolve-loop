# Build Explanation — Cycle 2300

## Build Binding
- Cycle: 2300
- Base SHA: 2675327d75d3f474f23cab4b1c24bc2fe8b42d0e

## Summary
The new verb `evolve cycle retire` retires a paused lane cycle. A retired cycle leaves the `cycle resume --all` list and cannot get a new resume grant.

## Rationale
Before this change, `cycle reset` sealed only the one cycle in the main state file. `cycle resume` granted a resume but did not revoke it. An abandoned lane stayed in the resume list for ever. The smallest fix disables the checkpoint block of the cycle. The paused-lane scan already skips a disabled checkpoint, so no scan code changes.

## Changed Areas
- `go/cmd/evolve/cmd_cycle.go` — routes the `retire` subcommand and lists it in the usage text.
- `go/cmd/evolve/cmd_cycle_retire.go` — checks the lane lease, finds the paused cycle, writes the ledger entry, disables the checkpoint and releases the inbox claims.
- `go/internal/checkpoint/checkpoint.go` — adds the `retired` reason as a valid checkpoint reason.
- `go/internal/checkpoint/autoresume.go` — adds `RetireCheckpoint`, which disables a checkpoint block under the state-file lock.
- `go/internal/checkpoint/autoresume_grant_test.go` — tests that a retired checkpoint stays disabled, even after a grant call.
- `go/internal/repocontract/repocontract.go` — adds `TestAmpCycleRetireRejectsBadInputWithoutChangingState` to the tree-reading test list, because that test reads the repository tree and the ship contract pack must run it.
- `docs/operations/runtime-reference.md` — documents the verb and its exit codes.

## Design Decisions
The verb refuses a live lane first (exit 3), then an unknown or not-paused cycle (exit 11). A refusal changes nothing. The ledger entry comes before the revoke, so a failed revoke leaves a retry possible. The inbox release uses the fixed reason `cycle-retired`. The worktree stays in place, because gc owns its removal.

Alternative rejected: extend `cycle reset`. It seals the single active cycle, not a named lane.

## Verification
The six ACS predicates of cycle 2300 pass. They cover the list, the grant refusal, the ledger seal, the live-lane refusal, the refusal paths and the claim release.

## Compatibility
No existing verb, flag or file format changes. The `retired` reason is new and only the retire verb writes it.

## Limitations
A ledger entry can exist for a cycle whose revoke failed. A second run of the verb adds a second entry.
