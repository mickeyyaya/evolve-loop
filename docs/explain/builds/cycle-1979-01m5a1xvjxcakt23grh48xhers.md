# Build Explanation — Cycle 1979

## Build Binding
- Cycle: 1979
- Base SHA: fa6b21ec97215af9a00f921cdf46d88bdfad5fff

## Summary
The new verb `evolve cycle reaudit --cycle N` rebases the worktree of cycle N onto the current main, runs the audit phase on the rebased tree and records an auditor ledger row that ship binds. A moved base no longer needs a blind resume or a hand-built audit request.

## Rationale
The ship recovery chain already rebases a lane and records conflicts, but only inside a recovery. One function, `core.ReauditCycle`, now holds the verb flow: load the cycle state, refuse a live run lease, rebase, audit, bind. The rebase step is `rebaseRecordingConflicts`. The verb and the debugger re-entry reach it through the variable `reauditRebase`, and the main fleet-rebase recovery in `ship_recovery.go` calls it directly. All three paths run the same function. The binding row comes from `auditBindingEntry`, the code that the orchestrator already used, so the verb and the loop record the same identity. The verb closes the signal publisher only after the audit has run.

## Changed Areas
- `go/internal/core/reaudit.go` — adds `ReauditCycle`, its input, result and typed error, and the shared `reauditRebase` step.
- `go/internal/core/phase_bindings.go` — extracts `auditBindingEntry` from `recordAuditBinding`. The loop behavior and the warning texts stay the same.
- `go/internal/core/ship_recovery_debugger.go` — the debugger re-entry calls `reauditRebase`.
- `go/cmd/evolve/cmd_cycle_reaudit.go` — adds the verb: flags, exit codes and the one-line result.
- `go/cmd/evolve/cmd_cycle.go` — registers the `reaudit` subcommand.

## Design Decisions
A live lease or a rebase conflict returns a typed error before any audit runs; the rebase aborts on a conflict, so the worktree stays as it was. The verb exits 1 for these and for infra errors, and 10 for usage errors. A FAIL verdict exits 0 because the verdict is data. Rejected: a second rebase routine in the verb, because two rebases drift apart. Rejected: soft-reset after the rebase, because the audit reads the committed rebased tree and the tests pin `HEAD^` to the new base. The cycle state file is not rewritten; the resume path owns that update.

## Verification
`go test -count=1 ./internal/core -run Reaudit`, `go test -count=1 ./cmd/evolve -run CycleReaudit` and the cycle 1979 ACS predicates pass.

## Compatibility
No flag or exit code of an existing verb changes. `recordAuditBinding` writes the same row as before.

## Limitations
The verb does not persist the new base into the cycle state and never ships.
