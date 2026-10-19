# cmd/testrusage

## Purpose

`cmd/testrusage` is a `go test -exec` wrapper. `go test` starts each test binary through it. The wrapper runs the test binary, passes its input, output and exit code through, and appends one resource record to the file that `TESTRUSAGE_OUT` names. Use it as `go test -exec "$PWD/bin/testrusage" ./...`, or through `make test-integration TEST_RUSAGE=1`.

## Design

- **`run` is the seam.** `main` registers the signal channel and calls `exitProcess(run(...))`. Tests replace `exitProcess` and pass their own streams and signal channel.
- **Pass-through.** The child gets the same stdin, stdout and stderr. The wrapper returns the exit code of the child. A child that a signal ended gives 128 plus the signal number. A child that does not start gives 127. No argument gives 2.
- **Signal forwarding.** SIGINT, SIGTERM and SIGQUIT go to the child, so a `go test -timeout` kill does not leave a test process behind.
- **`Record`.** The fields are `package`, `wall_s`, `user_s`, `system_s`, `peak_rss_mb`, `block_in`, `block_out`, `ctx_voluntary`, `ctx_involuntary` and `exit_code`. When the system gives no rusage, the CPU and RSS fields are zero and `note` is `rusage: unavailable`. The operating system adds the rusage of each waited child to its parent. So the record includes the `git` and `go` processes that the tests start.
- **`peakRSSMB`.** Linux gives `Maxrss` in kilobytes and macOS gives it in bytes. The function converts both to MB.
- **`packageName`.** The path of the working directory relative to the nearest `go.mod`. If no `go.mod` exists, it is the working directory.
- **`appendRecord`.** One `O_APPEND` write of one line per record, so concurrent packages do not interleave their lines.

## Invariants

- **The exit code is never changed by the telemetry.** A record write error prints a warning on stderr and the child exit code stays. Pinned by `TestRun_UnwritableOut_WarnsButKeepsChildExitCode` and `TestRun_FailingChild_PassesExitCodeThrough`.
- **The record sees the memory and the kernel time of the tree.** `TestRun_AllocAndSpawn_RecordsPeakRSSAndSystemCPU`: a child touches 64 MB and starts one subprocess. The record has `peak_rss_mb` of at least 64 and `system_s` above 0.
- **The memory unit is the same on both systems.** Pinned by `TestPeakRSSMB_NormalizesPerOS`.
- **Output is passed through exactly.** Pinned by `TestRun_PassesStdioThrough`.

## Findings

- Known limit, Ctrl-C: the wrapper and the child share one process group. A terminal Ctrl-C reaches the child twice: from the terminal and from the forward.
- Known limit, SIGKILL: a SIGKILL of the wrapper cannot be forwarded, so the child keeps running and has no parent.
- A wait that fails with no process state prints the error and exits 1, with no record.
- macOS gives 0 for `block_in` and `block_out`. Trust them on Linux only.

Design: [ci-telemetry-and-critical-path §5](../ci-telemetry-and-critical-path.md#5-level-2-the-resource-wrapper). Plan: component C0b of [ci-wall-time-2026-10](../../plans/ci-wall-time-2026-10.md).
