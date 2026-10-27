# Build Explanation — Cycle 2058

## Build Binding
- Cycle: 2058
- Base SHA: f8269cb929be8f9053d677959f8dc2d4d57e981e

## Summary
The new verb `evolve msg` has three sub-verbs: `send`, `wait` and `reply`. An agent uses them to talk to other agents of the same cycle. The verb writes a draft to the outbox of the dispatch and reads the receipt and the mailbox. The service of milestone M3 does the routing.

## Rationale
ADR-0129 section 11 states that the agent writes only its own outbox. The CLI therefore holds no broker. It writes a draft file, waits for the receipt file, and maps the answer to the exit table. A wait uses the kernel waker and no timer, so an idle agent costs nothing.

## Changed Areas
- `go/cmd/evolve/cmd_msg.go` — adds the three sub-verbs, the exit mapping, the caller from the environment, and the injected `msgEnv` for tests.
- `go/cmd/evolve/registry.go` — registers the `msg` verb in the command table.
- `go/cmd/evolve/cmd_msg_edges_test.go` — covers each local fault: stdin, entropy, draft write, arm refusal, malformed receipt and mailbox, write failures, and the environment caller.

## Design Decisions
The sender is never a flag. The service stamps it from the token. A `reply` reads the mailbox first, so an id outside the mailbox exits 4 and writes no draft. The service error arrives as text in the receipt, so exit 4 is chosen by the text of `broker.ErrNotFound`. Rejected: a poll loop for the receipt, because the source test bans timers.

## Verification
The 17 `TestMsg` tests pass, and every function of `cmd_msg.go` has 100% coverage. The test helper `msgEnvFor` in `cmd_msg_test.go` injects `time.Now` when the test uses the real waker. It uses the fixed clock (2026-10-27 10:00 UTC) only with a fake waker. The real waker returns a deadline at once for a past instant. See the build report.

## Compatibility
No existing verb changes. The new verb reads `EVOLVE_DISPATCH_ID` and `EVOLVE_MSG_TOKEN`, as the specification states.

## Limitations
`read` and `ack` are not in this task. A `wait` returns all messages of the mailbox, because the cursor moves only through `ack`.
