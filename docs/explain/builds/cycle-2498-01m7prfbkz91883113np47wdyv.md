# Build Explanation — Cycle 2498

## Build Binding
- Cycle: 2498
- Base SHA: d195ed6e358f5289942ce111e48e8127b927904d

## Summary
The agent run bridge now puts the directory of the running evolve binary first on PATH before it launches a seat.

## Rationale
A review seat pane did not find evolve on PATH. The seat inherits the process environment, so one PATH change in the launch path fixes every seat.

## Changed Areas
- `go/cmd/evolve/cmd_cycle.go` — adds `putEvolveBinaryDirOnPath`, called from `agentRunBridge`; it adds the binary directory once and adds no empty entry.
- `go/cmd/evolve/cmd_cycle_agentrun_path_test.go` — tests PATH order, the empty PATH case, a repeat call and the PATH at seat launch.
- `go/acs/cycle2498/predicates_test.go` — predicate that runs those tests.
- `.evolve/evals/review-seat-pane-has-no-evolve-on-path.md` — eval for the task.

## Design Decisions
PATH is read from `os.Environ()` because the envtaint predicate forbids an unregistered `os.Getenv("PATH")` and its allowlist is protected.

## Verification
The four `TestAgentRunBridge_EvolveBinaryDirOnPath*` tests pass.

## Compatibility
Host PATH entries stay after the evolve directory, in order.

## Limitations
A live `evolve agent run code-reviewer` seat run was not done in this cell.
