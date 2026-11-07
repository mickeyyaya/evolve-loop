# Build Explanation — Cycle 2214

## Build Binding
- Cycle: 2214
- Base SHA: 7a472e8c69c917b4c631567f7ab143439742471a

## Summary
The private tmux row of the sandbox contract test now keeps its server alive when the shell of the first session ends at once. The row starts the server, sets a stable default shell, and turns off exit-empty before it creates the session.

## Rationale
Under load the session shell can end before the window starts. Window 0 closes, the session goes, the server exits, and the next new-window fails with "no server running". A server that stays alive with exit-empty off removes this race at its source. The row had no readiness probe and no real-cause report, so the script now adds both. A has-session probe checks the kept session. When the probe fails, the text "the private tmux server never became ready" names the cause.

## Changed Areas
- `go/internal/bridge/sandbox_contract_integration_test.go` — the row script starts the server with start-server, default-shell /bin/sh and exit-empty off. It then creates the kept session and probes it with has-session. It reports the real cause when the server is not ready. Then it runs new-window.
- `go/internal/bridge/sandbox_contract_private_tmux_shell_integration_test.go` — adds the test where the session shell ends at once, which failed before the fix.
- `go/internal/bridge/sandbox_contract_private_tmux_integration_test.go` — adds the tests for a late client, a prompt client and a server that never starts.
- `go/acs/cycle2214/predicates_test.go` — adds the two acceptance predicates of this cycle.
- `.evolve/evals/bridge-sandbox-contract-private-tmux-flake.md` — adds the eval graders for the task.
- `docs/private/research/archived-2026-11-07/superseded-predicate-packages/cycle2201/predicates_test.go` — archives the predicate package of the unshipped cycle 2201.
- `docs/private/research/archived-2026-11-07/unshipped-build-explanations/cycle-2201-01m6rb41b9yebnpsm70kn83h68.md` — archives the build explanation of the unshipped cycle 2201.

## Design Decisions
The existing test file changes only in the script line; the cycle also adds test files, predicates and archived documents. A longer wait can hide the race, and exit-empty off alone fails with "no current target" when no session remains, so both parts are needed.

## Verification
The cycle-2214 predicates pass. The ACS suite reports green=265 red=0. The private tmux rows pass 50 runs in a row.

## Compatibility
The row gains a readiness probe and a failure text; the contract it asserts on the window tty is unchanged. No production code changes.

## Limitations
The sandboxed variant of the row cannot run in this pane, so the measured row is the unsandboxed one.
