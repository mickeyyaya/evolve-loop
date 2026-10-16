# internal/logfilter

## Purpose

`internal/logfilter` writes a human-readable `<phase>-stdout.clean.txt` next to each raw `<phase>-stdout.log` that the bridge captured. The raw file is never touched.

## Design

- **One entry point for all drivers.** `Process` classifies each line by a JSON parse first. A line is stream-json (claude-p, codex, agy) or plain text (claude-tmux). No format-mode flag is needed. `Process` reads the raw file and writes the clean file atomically, through a temporary file and a rename. It returns nil when the raw file is missing, because the phase never reached the bridge.
- **No signal is lost.** A blank line is neither JSON nor signal and is dropped. A line that looks like JSON but does not parse goes to the plain-text path. An unknown event type or system subtype keeps one compressed line of the raw text.
- **Drop and keep.** `classifyJSON` returns `handled=true` with an empty text to mean "drop this line". The `stream_event` partial deltas are dropped, because the assembled assistant event makes them redundant. A `rate_limit` event is small but useful, so it keeps one compact line.
- **Tool results.** The content can be a JSON string or a nested array or object. It is rendered as a string and truncated either way. A truncation keeps the head and an elision marker with the count of elided bytes.
- **Plain-text dedup.** `plainTextState` collapses consecutive identical lines into one `(× N times)` line. Spinner redraws and repeated hook noise are such lines. A noise line breaks the streak and emits the pending run. `flush` returns the pending run. It runs at the end of the stream and at each change to a JSON line.
- **Noise.** Noise means a blank line, a line of only spinner or progress glyphs, or a line of only box-drawing borders. The spinner glyphs are the braille patterns of the Claude Code TUI plus a few ASCII rotators. The borders are the box-drawing characters of its bordered panels.
- **Line length cap.** `maxScannerBufBytes` is 16 MB, the same cap as `cyclecost`, so that stream-json events with deeply nested usage figures do not break the line scanner. A longer line gives `bufio.ErrTooLong`.
- **Errors leave no residue.** `filterStream` has one flush at its end, and every error path returns before it. Partial output after an error is not wanted. A failure in the filter step or in the rename removes the temporary file and returns an error that names the step (`open raw`, `filter`, `rename`).

## Invariants

- **A temporary file never survives.** Pinned by `TestProcess_RenameErrorWhenCleanPathIsDirectory` and `TestProcess_FilterErrorCleansUp`.
- **Every write error propagates.** The tests in `edges_test.go` force a write failure at each emit point. `bufio.Writer` has a default buffer of 4096 bytes. A write larger than that goes straight to the underlying writer, so the tests size their lines around that value.
- **A blank line flushes a pending run.** In the full pipeline `classifyJSON` takes whitespace lines before they reach the plain-text state, so `TestPlainTextState_IsNoiseBlankFlushesPendingRun` pins the behavior at the unit level.

## Findings

- None.
