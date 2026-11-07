# Build Explanation — Cycle 2201

## Build Binding
- Cycle: 2201
- Base SHA: 08bed240e88931ef62a1f1decbb9c91df3a3c6cc

## Summary
The private tmux contract row now keeps its server alive with a keeper session and checks that the server is ready.

## Rationale
The window command ended and took the only session with it. The server then exited, and the late `wait-for ran` found no server. A keeper session holds the server open until the row kills it.

Alternative not taken: turning the tmux `exit-empty` option off can also keep the server alive. The triage report named it. The keeper session was chosen because it needs no server option, and the row keeps the same server configuration as production use.

## Changed Areas
- `go/internal/bridge/sandbox_contract_integration_test.go` — the row starts a keeper session, probes it with `has-session`, then runs the window with `new-window`.
- `go/internal/bridge/sandbox_contract_private_tmux_integration_test.go` — tests for a late client, the readiness wait, a server that never starts, and a prompt client.
- `go/acs/cycle2201/predicates_test.go` — predicates that run those tests.
- `.evolve/evals/bridge-sandbox-contract-private-tmux-flake.md` — the eval graders for this task.

## Design Decisions
The probe is `has-session`, so the row uses no sleep. A failed probe prints that the private tmux server never became ready. The `exit-empty` option was the other choice (see Rationale).

## Verification
The four `TestPrivateTmuxRow` tests pass. Only the late-client test (`go/internal/bridge/sandbox_contract_private_tmux_integration_test.go:84`) uses a shim that delays each tmux client. The absent-server test (line 107) uses a different shim that never starts a server. The readiness test and the prompt-client test use no shim.

## Compatibility
The row keeps its action name and its expected verdict.

## Limitations
The literal acceptance criterion of 50 runs under a loaded host did not run in this build. `TestAgentSandboxContract` skips where `sandbox_apply` is refused, so the row itself was not exercised end to end here. The shims in the four tests are the evidence for the fix. The row still depends on a local tmux binary.
