# Build Explanation — Cycle 1794

## Build Binding
- Cycle: 1794
- Base SHA: 0f072732bb7a4493220ded7239c3a20758687d94

## Summary
`evolve signals tail [--cycle N] [--kind K,…] [--code C,…] [--follow] [--json]` is a new read-only subcommand. It prints Signal Center streams in time order. `--cycle N` reads one cycle's `signals.ndjson`. Without it, the tail reads the live wave: every run dir whose run lease is live, merged by parsed instant. With `--follow` it prints appended events and ends by itself once the cycle or the wave is sealed. Two pure primitives back it in `signalcenter`: `ReadStream` and `MergeByTS`.

## Rationale
Before this change, the only way to watch a wave was to `tail -f` each `.evolve/runs/cycle-*/signals.ndjson` by hand. That mis-orders events, because RFC3339Nano trims trailing zeros (`…05.1Z` sorts before `…05Z` as a string). The api-contract (D1–D9) settled the design:
- The live wave is `runlease.LiveRuns`, not "unsealed streams", because 9 of the 12 unsealed streams measured on disk were stale crashed cycles.
- Ordering parses the instants instead of comparing strings.
- `signalcenter` stays a stdlib-only leaf. Lane selection and the follow loop live in `cmd/evolve`.

## Changed Areas
- `go/internal/signalcenter/stream.go` — adds `StreamChunk`, `ReadStream` and `MergeByTS`.
  - `ReadStream` reads one NDJSON stream from a byte offset and consumes only complete lines, so a half-written line is returned once its newline lands. It skips and counts malformed lines and events whose `ts` does not parse. It re-reads from 0 when the offset is negative or past the end of a shrunk file. A missing file is `fs.ErrNotExist`, and any other I/O fault (for example a directory) is a separate error.
  - `MergeByTS` merges event slices into a fresh slice, stably, by parsed instant. Unparsable timestamps sort first.
- `go/internal/signalcenter/stream_test.go` — behavior tests for both primitives that keep the package at its 100% `.cover-strict` floor: fragment held, offsets clamped, the missing-file vs I/O-fault split, and instant order with stability and purity.
- `go/cmd/evolve/cmd_signals_tail.go` — the `signals tail` subcommand.
  - Flag parsing: `--kind` and `--code` take comma sets validated by `Kind.Known` / `Code.Valid`. A bad flag, a positional, `-h` or `--cycle < 1` exits 10.
  - Stream selection: `--cycle N`, or the live wave with a fallback to the numerically newest strict `cycle-<int>` stream.
  - Output: `TS + " " + FormatLine` per line, or one `signal/1.0` JSON line each with `--json`.
  - Follow: polls every second, joins newly live lanes, adds root-stream appends but never its history, and ends on `cycle.sealed` even when a filter hides it.
  - Exit codes: 0 ok, 1 missing `--cycle` stream, 2 I/O fault, 10 usage.
- `go/cmd/evolve/cmd_signals_tail_test.go` — reachability through `runSignals`, usage errors, wave follow that ends on the seal, follow that ends on cancel, and the missing-cycle exit 1.
- `go/cmd/evolve/cmd_signals.go` — `runSignals` dispatches `tail` with a SIGINT/SIGTERM `NotifyContext`, and the usage line advertises the tail. The `codes generate|check` paths are unchanged.
- `docs/operations/runtime-reference.md` — one operator-command entry: the flags, wave selection, follow end conditions, exit codes, and the missing `ship.landed` producer.

## Design Decisions
- The follow state (offsets, sealed and gone flags) lives in a small `signalsTail` struct in cmd. `signalcenter` exports only pure read and merge primitives, so the leaf import rule holds.
- `ReadStream` gets the size with `Seek(SeekEnd)`, then seeks to the start and reads to EOF. Reading a directory always reaches `read(2)` and fails with EISDIR, so a directory can never read as an empty stream.
- In wave mode the root stream is primed on the first poll, whether or not it exists yet. A root stream created mid-follow therefore has all its lines printed instead of being treated as history.
- A lane that drops out of `LiveRuns` is read once more before it counts as done, so its last events are not lost.

## Verification
- `cd go && go test -tags acs -count=1 ./acs/cycle1794/`: 20/20 predicates pass.
- `go test -count=1 ./cmd/evolve/ ./internal/signalcenter/` passes. `stream.go` is at 100% coverage, and the package total is 100%.
- `gofmt -l .` is empty and `go vet` is clean.

## Compatibility
`evolve signals codes generate|check` and its exit 10 usage paths behave as before. Only the usage text gained the tail form. The signal wire schema and `FormatLine` are reused unchanged, and nothing under the project is written.

## Limitations
- `ship.landed` has no production producer, so `--kind ship.landed` prints nothing on live data until a landing producer exists.
- Wave `--follow` ends at the end of a wave, so a multi-wave batch needs one tail per wave.
- Polling is fixed at one second in production.
