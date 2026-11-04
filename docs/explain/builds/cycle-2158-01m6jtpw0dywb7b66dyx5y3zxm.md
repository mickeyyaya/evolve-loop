# Build Explanation — Cycle 2158

## Build Binding
- Cycle: 2158
- Base SHA: c9a7ef419e00a8428c534b2644969bc454d13669

## Summary
The sandbox contract test helper that runs the agent command now bounds its wait and removes private tmux servers.

## Rationale
The call used CombinedOutput with no wait limit. A daemon child that kept stdout open blocked the call until the package timeout. A WaitDelay of two seconds ends the wait, and the output written before then stays. A deferred sweep of tmux sockets under the row scratch directory removes servers that the row leaves behind.

## Changed Areas
- `go/internal/bridge/sandbox_contract_integration_test.go` — sets the wait delay on the agent command and reaps tmux servers whose socket lies in the row scratch directory.
- `go/internal/bridge/sandbox_contract_waitdelay_integration_test.go` — adds two tests: return within 15 seconds with a daemon child, and no private tmux server after the row.
- `go/acs/cycle2158/predicates_test.go` — adds the acceptance predicates of the cycle.
- `.evolve/evals/sandbox-contract-agent-waitdelay.md` — adds the eval graders of the task.

## Design Decisions
The sweep walks the scratch directory for socket files, so it needs no knowledge of the socket name that a row picks. A process-group kill was rejected because the tmux server starts its own session.

## Verification
Five consecutive runs with -count=1 and the integration tag pass for the two new TestAgentWaitDelay tests. TestAgentSandboxContract skips on this runner, so the five-run criterion for the tmux -L contract row is not verified here. The ACS suite reports green=266 red=0.

## Compatibility
Only test code changes. Production code is unchanged.

## Limitations
A tmux server with a socket outside the row scratch directory is not removed.
