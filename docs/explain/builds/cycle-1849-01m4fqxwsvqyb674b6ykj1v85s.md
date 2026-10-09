# Build Explanation — Cycle 1849

## Build Binding
- Cycle: 1849
- Base SHA: ba41c8b976dd58aa1d197cc7d6ef2a02a4c11282

## Summary
Four dashboard rules that survived deletion in every test are now pinned by tests proven through mutation: the zero `WriteTimeout` on the dashboard server, the `seq <= last` echo filter on the SSE stream, the board lane-status overlay in the cycle detail endpoint, and the cap that keeps only the newest run workspaces. The two kept why-comments those tests replace are deleted. No production behaviour changes.

## Rationale
Comment round 12 found two prose comments standing in for missing tests and two rules no test would notice losing. A test that fails when its rule is removed replaces the comment as the record of intent, and the cycle's ACS predicates remove each rule through a `go test -overlay` copy to prove the kill.

## Changed Areas
- `go/internal/dashboard/server.go` — deletes the `WriteTimeout` why-comment; the rule is now pinned by `TestServer_SSEStreamOutlivesAnyWriteDeadline`.
- `go/internal/dashboard/sse.go` — deletes the subscribe-order why-comment; the echo filter it explained is now pinned by `TestServer_SSEFirstSnapshotIDIsNeverRepeated`.
- `go/internal/dashboard/server_test.go` — adds the stream-lifetime test (a real `Serve` stream still delivers keep-alive pings after 400 ms), the first-snapshot test (a server with no snapshot publishes its first build into the just-subscribed channel and the stream must not repeat that id) and the board-overlay test (a non-primary running fleet lane reads running from the board rather than incomplete from the loop card).
- `go/internal/dashboard/collect_test.go` — adds the cap test: four plain workspaces and one running lane at cap 2 render the two newest workspaces plus the running lane.
- `docs/architecture/packages/internal-dashboard.md` — names the new pinning tests and drops the notes that said no test pinned these rules.
- `go/acs/cycle1849/predicates_test.go` — the TDD phase's mutation-kill predicates for the four rules and the comment removal, committed unchanged.
- `.evolve/evals/dashboard-sse-and-board-status-pins.md` — the TDD phase's eval for this task, committed unchanged.

## Design Decisions
Each test drives the real entry point (`Serve`, `Handler`, `collect`) rather than the helper it pins, so the mutation of the production line is what the test observes. The first-snapshot test uses the nil-snapshot route because it makes the echo deterministic: the publish lands in the subscriber's channel before the select loop runs, so without the filter the repeated id is written before the first keep-alive ping.

## Verification
`go test -tags acs -count=1 ./acs/cycle1849` is 6/6 PASS, including the four mutation kills. The four new tests pass under `-race -count=5`, and the full `./internal/dashboard/...` suite passes.

## Compatibility
Test and documentation changes plus two comment deletions; the dashboard's runtime behaviour and API are unchanged.

## Limitations
The subscribe-before-`current()` ordering itself is not pinned: losing a publish between the two calls cannot be forced without a test hook, which this cycle does not add.
