# cmd/testlatency

## Purpose

`cmd/testlatency` turns a `go test -json` event stream into a Markdown latency report. The report has the wall time of each package, the longest test of each package, the slowest tests overall, and counts over configurable thresholds. It reads the stream on stdin and writes Markdown to stdout, so it composes with the Makefile: `go test -json ./... | go run ./cmd/testlatency -title "Fast suite" > report.md`.

## Design

- **`event`** holds the subset of `go test -json` fields that the tool consumes. The full schema is in `go doc test2json`.
- **`pkgStat`.** `Wall` is the package-level `Elapsed`, the real wall time that accounts for parallelism. `AllTestsSum` is the sum of the top-level test `Elapsed`, the fully serial upper bound. `SerialOnlySum` and `ParallelLongest` split that time by the `pause` action (see the next item). `Status` is `pass`, `fail`, `skip` or empty.
- **Serial and parallel split.** A `pause` event marks a top-level test as parallel. `SerialOnlySum` is the sum of `Elapsed` of the top-level tests that never emit `pause`. `ParallelLongest` is the largest `Elapsed` of the top-level tests that emit `pause`.
- **Floor.** `Floor` is `SerialOnlySum` plus `ParallelLongest`. This is the least wall time that the package needs with unlimited parallelism. `Dominant` names the larger part, `serial` or `parallel`. A tie reports `serial`.
- **Pause scope.** The tool keys a pause by package and test name. A pause on a subtest does not make the parent parallel. The tool clears the mark at the terminal event, so a repeated run (`-count=N`) starts serial.
- **Paused test without terminal event.** The test adds no time to either part. The package has no terminal summary, so it appears in `Incomplete`.
- **Top-level tests only.** A name with `/` is a subtest. `Parse` excludes subtests from the test list and from every sum, because the parent already contains their time.
- **Stable sort.** Packages and tests with equal timing keep a reproducible order.
- **Large lines.** The scanner buffer grows lazily to 16 MiB, because some `Output` lines are large.
- **Report sections.** Optimization targets (packages over the wall threshold) with the columns Package, Wall, Tests, Σall tests, Serial Σ, Parallel max, Floor, Dominant, Slowest test and Slowest.
- **Table columns.** The Σall tests column sums every top-level test. The Serial Σ column sums only the tests without `pause`.
- **Package budget.** `--budget` leads the report with the packages over a wall time. `--fail-over-budget` then exits 1. `-package-budget` sets both from `addedtests.PackageWallBudget`. `make test-budget` runs it on the fast suite.
- **Duration ratchet.** `-ratchet <file> -rusage <ndjson>` reads `go/.test-latency.json` and the package CPU of `test-rusage.ndjson`. The command exits 1 for a package above its CPU baseline after the speed index, or with a wall above `-test-timeout`. The default of `-test-timeout` is `20m`, the `TEST_TIMEOUT` of the Makefile. A missing `-rusage`, or a ratchet file or rusage file that is missing or malformed, gives exit 2 and names the file.
- **CPU judgment.** The file has a `cpu` map for each package, keyed by the OS name. A baseline of another OS never gates the host.
  - The package CPU is `user_s + system_s`. A reference package has a baseline of 2 s or more.
  - The speed index S of a package is the median ratio of run CPU to baseline over the other reference packages.
  - A package fails when its CPU divided by S is above 1.35 times its baseline and more than 15 s over it.
  - A package with fewer than 3 other reference packages appears in the section `## Package CPU against the recorded baseline` as reported only. It never fails.
  - The `class` field stays in the file and must be a known class. It no longer sets a wall limit.
  - Research: [ci-wall-variance-2026-11](../../research/ci-wall-variance-2026-11.md).
- **p90 warning.** With `-ratchet`, the report lists each package whose wall time is above its recorded p90 by 50% and by more than 30 seconds, under `## Packages above their recorded p90`. A warning never changes the exit code.
- **`tighten`.** `testlatency tighten -ratchet <file> [-rusage <ndjson>]` lowers the p90 of a recorded package that ran faster. With `-rusage`, it also lowers the CPU baseline of this OS, or seeds it when the package has none. It never raises a value and never adds a package. It does not rewrite the file when nothing is lower. Only a reviewed diff of the file raises a value.
- **Timeout margin.** `-timeout-margin <duration>` adds the section `## Margin against the go test timeout`.
  - The section has one row for the slowest package with a terminal summary: wall, timeout and margin in seconds and percent.
  - It has one row for each package with no terminal summary.
  - A row is `LOW` when the margin is under 20% of the timeout.
  - The section never changes the exit code. A value that is not a positive duration gives exit 2.
  - The macOS job reads the duration from `make print-test-timeout-darwin`, the one home of the number.
- **Other sections.** The report also has the slowest tests (capped by `Top`). A count of tests over the per-test threshold is the regression watch. `shortPkg` trims the module prefix.

## Invariants

- **An incomplete package is visible.** A package that starts but never emits a terminal pass, fail or skip summary (truncated stream, panic, build or timeout kill) is recorded in `Incomplete`. The Markdown notes it, so a partial run is never mistaken for a complete one. Pinned by `TestParse_FlagsIncompletePackage`.
- **A broken line is skipped.** A line that starts with `{` but is not valid JSON, and non-event lines such as build output, are tolerated. The valid events around them are still aggregated.
- **A read error is surfaced.** `Parse` returns a stream read error. It never returns a silently partial report. The scanner error path is the only place a malformed pipe can be reported.
- **An empty slow table has a placeholder.** When no package exceeds the threshold, the table shows the `_(none)_` row. Readers scan for that row.
- **`Top` caps the slowest-tests table.**
- **`-package-budget` fails over the named budget.** It exits 1 when a package wall time exceeds `addedtests.PackageWallBudget`, and 0 at or under it. `TestRun_PackageBudgetFlagUsesNamedWallBudgetAndFailsOverIt` pins this.

Design: [ci-telemetry-and-critical-path §4](../ci-telemetry-and-critical-path.md#4-level-1-and-level-2-the-test-stream). Plan: component C0 of [ci-wall-time-2026-10](../../plans/ci-wall-time-2026-10.md).
