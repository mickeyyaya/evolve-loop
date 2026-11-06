# Build Explanation — Cycle 2185

## Build Binding
- Cycle: 2185
- Base SHA: a5f971c2d806906c3af71e3deb3c0ba2393f8a67

## Summary
The branch adds the turn sidecar `llm-turns.ndjson`. Each assistant message of a claude transcript and each agy generation becomes one line. The line is keyed by the call id of the llm-calls row. This build also fixes the package note that still described the removed tool-name rule.

## Rationale
The row of a call hides how tokens spread over its turns. A sidecar keeps the row as one line and still shows the spread. The sidecar is telemetry, so a failure of it warns and never changes the launch or the row.

## Changed Areas
- `go/internal/tokenusage/turns.go` — builds one turn for each message id of a claude transcript, in time order. Tool names come from all in-window lines of the id, and a repeated tool_use line counts once.
- `go/internal/tokenusage/scanner.go` — the scanner now keeps the turns of a transcript in its result, so the row and the turns come from one scan.
- `go/internal/tokenusage/agy.go` — the agy reader gives one turn for each generation row. A malformed row is not a turn, so the turns sum to the row.
- `go/internal/llmcalls/turns.go` — appends the turn lines to the sidecar in the workspace. It writes at most the line cap for each call and adds a truncated count above the cap.
- `go/internal/bridge/attempt_telemetry.go` — writes the sidecar after the row of a launch. It compares the turn sum with the row, warns on a difference and leaves the row unchanged. A write failure warns and never fails the launch.
- `go/internal/bridge/signal.go` — adds the signal codes for the turn warnings, so each warning has a named code.
- `docs/architecture/packages/internal-tokenusage.md` — the turn rule says that tool names come from all in-window lines of a message id.
- `docs/architecture/packages/internal-llmcalls.md` — records the sidecar, its line cap and its truncated count.
- `docs/architecture/signal-codes.md` — lists the new turn signal codes.
- `go/acs/cycle2185/predicates_test.go` — predicates 001 to 006 pin the acceptance of the sidecar.

## Design Decisions
The sidecar is separate from the row. The row stays one line for each call, so existing readers do not change. The line cap bounds the file size.

## Verification
`go test -tags acs ./acs/cycle2185` passes. The ACS suite for the cycle reports green=264 red=0. Unit tests in the tokenusage, llmcalls and bridge packages cover each path.

## Compatibility
The row format is unchanged. The sidecar is a new file, and a call with no turns writes no file.

## Limitations
Finding L1 of cycle 2175 stays open. Turn lines can be written after a failed row append. It is LOW and outside the acceptance.
