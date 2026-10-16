# cmd/testlatency

## Purpose

`cmd/testlatency` turns a `go test -json` event stream into a Markdown latency report. The report has the wall time of each package, the longest test of each package, the slowest tests overall, and counts over configurable thresholds. It reads the stream on stdin and writes Markdown to stdout, so it composes with the Makefile: `go test -json ./... | go run ./cmd/testlatency -title "Fast suite" > report.md`.

## Design

- **`event`** holds the subset of `go test -json` fields that the tool consumes. The full schema is in `go doc test2json`.
- **`pkgStat`.** `Wall` is the package-level `Elapsed`, the real wall time that accounts for parallelism. `SerialSum` is the sum of the top-level test `Elapsed`, the fully serial upper bound. `Status` is `pass`, `fail`, `skip` or empty.
- **Top-level tests only.** A name with `/` is a subtest. `Parse` excludes subtests from the test list and from the serial sum, because the parent already contains their time.
- **Stable sort.** Packages and tests with equal timing keep a reproducible order.
- **Large lines.** The scanner buffer grows lazily to 16 MiB, because some `Output` lines are large.
- **Report sections.** Optimization targets (packages over the wall threshold), and the slowest tests (capped by `Top`). A count of tests over the per-test threshold is the regression watch. `shortPkg` trims the module prefix.

## Invariants

- **An incomplete package is visible.** A package that starts but never emits a terminal pass, fail or skip summary (truncated stream, panic, build or timeout kill) is recorded in `Incomplete`. The Markdown notes it, so a partial run is never mistaken for a complete one. Pinned by `TestParse_FlagsIncompletePackage`.
- **A broken line is skipped.** A line that starts with `{` but is not valid JSON, and non-event lines such as build output, are tolerated. The valid events around them are still aggregated.
- **A read error is surfaced.** `Parse` returns a stream read error. It never returns a silently partial report. The scanner error path is the only place a malformed pipe can be reported.
- **An empty slow table has a placeholder.** When no package exceeds the threshold, the table shows the `_(none)_` row. Readers scan for that row.
- **`Top` caps the slowest-tests table.**
