# internal/log

## Purpose

`internal/log` holds the logging surfaces of the codebase. It writes structured events into JSONL sidecar files (`EmitAbnormal`, `SidecarWriter`) and into `slog` channels (`EmitPhase`, `NewJSONLogger`). `Console` prints operator-facing prose. `DiagnosticField` and `SanitizeField` render untrusted text safely. All are safe for concurrent use.

## Design

- **`EmitAbnormal` writes the line shape of `abnormal-events.jsonl`.** The shape is `{event_type, timestamp, ...fields}`, with `severity` when present. Fields are flattened beside `event_type`.
- **The old corpus has two shapes.** `subscription-auth-mode` has `ts`, `cycle`, `mode` and `notes`. `stall-detected` has `timestamp`, `source_phase`, `severity`, `details` and `remediation_hint`. The Go port uses `timestamp` (ISO 8601 UTC, second precision).
- **A zero `Timestamp` defaults to `time.Now().UTC()`.** `now` is the test seam.
- **`SidecarWriter` opens and closes the file for each emit.** A long-running process leaks no file descriptor. Rotation by an external tool keeps working. `Close` is therefore a no-op and is idempotent. A later reuse of an `O_APPEND` descriptor needs no contract change.
- **`SidecarWriter` creates the parent directory on the first emit.** A mutex serializes each write of one newline-terminated buffer. A line never tears.
- **`appendLine` is a test seam.** A real append fails only when the disk is full. The seam lets `events_coverfix_test.go` prove that the error names the path. `NewSidecarWriter` is the required constructor, because a bare `SidecarWriter{}` literal leaves `appendLine` and `now` nil.
- **`marshalSorted` writes keys in lexical order.** Lines stay easy to diff. The helper controls the order itself. A change in the map order of `json.Marshal` does not change the output. A custom encoder, such as compact number formatting, can replace it.
- **`EmitPhase` records a lifecycle event into a `slog.Logger`.** The shape is `{"phase":..., "event":..., ...extra}` plus the time and level fields of slog. When `extra` collides with `phase` or `event`, `extra` wins. This is a deliberate trust call, because the caller controls both sides.
- **The tests do not pin the exact `EmitPhase` handler output.** It depends on the version of the standard library.
- **`NewJSONLogger` takes a case-insensitive level.** An unknown level falls back to info. The logger has no source attribute, so each line stays compact for log aggregation.
- **`Console` is the one human-facing logger.** It concentrates the scattered `fmt.Fprintf(os.Stderr, ...)` printers behind one injectable seam. `Info` goes to `Out`. `Warn` and `Error` go to `Err`.
- **`Default` wires the process streams.** It serves CLI output, where `Info` belongs on stdout. `Diag` sends every level to stderr, `Info` included. That is the Unix convention: stdout holds program data.
- **`Console` covers prose only.** The structured emitters are unchanged. The zero value is safe, because a nil sink is a no-op. Tests inject `*strings.Builder` sinks.
- **One bound serves both field renderers.** `boundedUTF8` repairs invalid bytes to the replacement rune. It caps the text at `maxDiagnosticRunes` runes with a trailing ellipsis.
- **The two renderers differ only in the output form.** `DiagnosticField` quotes the result to ASCII. `SanitizeField` keeps one line without quotes. It serves values inside a JSON string or a single-line record, such as the event fields and reasons of the Signal Center (ADR-0101). The renderers never disagree on the length of a field. `sanitize_test.go` pins this.
- **`NewSelectionLogger` is the one writer factory.** It builds a JSON handler and a text handler from `Sinks`. Both get the same record. `WithSelection` puts a `Selection` into the context.
- **The factory adds the selection keys.** The logger adds the non-zero keys `run`, `cycle`, `phase`, `dispatch`, `cat`, `component` and `code`. The handlers write `ts` in place of the slog key `time`, and `level` always. A record attribute with a selection key wins.
- **`Sinks.Render` swaps the text handler for a line writer.** A moved loop-log writer keeps its text (`signalcenter.StderrSink`).

## Invariants

- **`Fields` cannot override `event_type` or `timestamp`.** The canonical keys always win. `TestEmitAbnormal_FieldsCannotOverrideCanonical` pins this.
- **`Warnf` and `Errorf` are never suppressed.** `Quiet` stops only `Info`. A suppressed warning is worse than a noisy one. `console_test.go` pins this.
- **`EmitAbnormal` returns an error only for a missing `EventType` or an I/O failure.**
- **Untrusted text never reaches a log line raw.** `DiagnosticField` never returns a raw line break, a terminal control character, an invalid UTF-8 byte or a Unicode formatting character.
- **`SanitizeField` turns hidden characters into a space.** They are control characters, `Cf` format characters (bidi overrides, zero-width joiners, the BOM) and the Unicode line and paragraph separators. A rendered line must read left to right as written. `TestDiagnosticField_QuotesAndBoundsUntrustedText` and `sanitize_test.go` pin this.
