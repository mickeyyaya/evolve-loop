# Build Explanation — Cycle 1845

## Build Binding
- Cycle: 1845
- Base SHA: 30c5872f7f5ac4045167af3dc654d12127e14c33

## Summary
The CI watch polling loop now bounds every fetch operation by the remaining watch deadline, mapping hung or timed-out fetch attempts to `ErrWatchTimeout` with the last observed run status.

## Rationale
When a background `gh` command or network request stalls indefinitely during a CI watch operation, unbound fetch calls prevent the watch deadline from terminating the process. Bounding each fetch call by the remaining deadline ensures deterministic termination with exit code 2 and diagnostic output on stderr.

## Changed Areas
- `go/internal/ciwatch/ciwatch.go` — bounds each fetch call by the remaining watch deadline, ensures `ErrWatchTimeout` returns the last observed status (defaulting to queued), and propagates deadline timeouts deterministically.
- `go/internal/ciwatch/ciwatch_test.go` — verifies that hanging fetch operations time out at the deadline and report the last observed status.
- `go/cmd/evolve/cmd_ci_watch_test.go` — verifies command entrypoint argument validation and error exit handling.

## Design Decisions
The fetch timeout duration is dynamically derived from the remaining watch budget rather than an arbitrary sub-timeout, ensuring tests with small watch timeouts expire promptly while standard runs enjoy full configured deadlines.

## Verification
Unit tests verify timeout when fetch hangs in `ciwatch_test.go`, command error paths are verified in `cmd_ci_watch_test.go`, and ACS suite predicates `TestC1845_001` through `TestC1845_006` verify end-to-end command exit codes with fake `gh` executions.

## Compatibility
Existing CLI flags, exit code partitions (0 for green, 1 for red, 2 for unobservable/timeout, 10 for usage), and policy configurations remain unchanged.

## Limitations
This change does not retry stalled fetch calls within a single poll cycle.
