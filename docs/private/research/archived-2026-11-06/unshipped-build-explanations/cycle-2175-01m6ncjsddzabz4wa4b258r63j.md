# Build Explanation — Cycle 2175

## Build Binding
- Cycle: 2175
- Base SHA: e69d7f45bb52d67e4150fd401a58d66dd5390717

## Summary
Each call now leaves one turn line in a sidecar file, `llm-turns.ndjson`. A line holds the token counts, the tool names and the context size of one turn. An agy turn has no tool names. The line carries the `call_id` of the matching `llm-calls` row.

## Rationale
The row of a call holds only the sum of its tokens. A sidecar keeps the per-turn detail and leaves the row at one line for each call. The collectors already read each turn to build the sum, so they now also return the turns. The sum of the turns equals the row.

## Changed Areas
- `go/internal/tokenusage/turns.go` — adds the `Turn` type and the ordering of claude messages by time. `addTools` collects the tool names over all lines of one message id, in order, and skips a repeated tool id.
- `go/internal/tokenusage/scanner.go` — keeps the timestamp and the tool names of each assistant message, and returns them in `Result.Turns`.
- `go/internal/tokenusage/agy.go` — adds one turn for each valid generation row of an agy conversation. The turn has no tool names, because the generation row holds none.
- `go/internal/tokenusage/scanner_mtime_filter_test.go` — compares two results with `reflect.DeepEqual`, because `Result` now holds a slice.
- `go/internal/llmcalls/turns.go` — adds `AppendTurns`, `TurnsPath` and the cap `MaxTurnLines`. A call above the cap keeps the first turns and adds one line with a `truncated` count.
- `go/internal/bridge/attempt_telemetry.go` — writes the sidecar after the row append, and warns when the turn sum differs from the row or when the write fails.
- `go/internal/bridge/signal.go` — registers the signal codes `BRIDGE_TURN_SUM_MISMATCH` and `BRIDGE_TURN_APPEND_FAILED`.
- `docs/architecture/packages/internal-tokenusage.md` — records the turn invariant.
- `docs/architecture/packages/internal-llmcalls.md` — records the sidecar design.

## Design Decisions
The sidecar reuses the locked append of the row ledger, so it gets the same torn-tail protection. A mismatch only warns and never changes the row. A write failure only warns and never fails the launch, because the sidecar is telemetry. The rejected alternative is to put the turns inside the row: it makes the row large and breaks the one-line-for-each-call rule.

## Verification
Four test files cover the five acceptance criteria. They are `go/internal/tokenusage/turns_test.go`, `go/internal/llmcalls/turns_test.go` and `go/internal/bridge/attempt_turns_test.go`. The fourth is `go/internal/tokenusage/turns_parallel_tools_test.go`. It proves that parallel tool calls of one message id keep every tool name in order. The cycle ACS predicates run the same tests.

## Compatibility
The row schema is unchanged. Old readers ignore the new file. A result without turns writes no sidecar.

## Limitations
An agy turn has no timestamp and no tool names, because the generation row holds neither. The cap is 500 lines for each call.
