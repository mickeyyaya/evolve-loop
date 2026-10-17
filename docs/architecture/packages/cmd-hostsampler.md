# cmd/hostsampler

## Purpose

`cmd/hostsampler` samples the whole host once each interval (default 1 s) and appends one NDJSON record per sample to the `-out` file. It runs in the background for the Go job and runs until the job ends. Nothing stops it: the runner discards the process with the job, and SIGTERM or SIGINT still end it cleanly. The records show if a slow package waits for a free CPU or for memory.

## Design

- **`Sample` fields.** `ts`, `cpu_busy_pct`, `runqueue`, `mem_used_mb`, `swap_used_mb` and `disk_busy_pct`. A field that the host cannot give, or that a tool failed to give in this tick, is absent, not zero.
- **Linux sources.** `/proc/stat` gives the CPU ticks and `procs_running`. `/proc/meminfo` gives the memory: used is `MemTotal` minus `MemAvailable`, and swap is `SwapTotal` minus `SwapFree`. `/proc/diskstats` gives `io_ticks` of the whole disks `sd*`, `vd*`, `xvd*`, `nvme*n*` and `mmcblk*`. The parsers take text and `readLinux` takes the root directory, so tests use a fixture directory.
- **macOS sources.** No cgo. `source_darwin.go` runs `vm_stat` (used is active plus wired plus compressor pages) and `sysctl -n vm.swapusage`, through `sysexec.Command`. Each tool has a deadline of 800 ms, which is shorter than the default interval. The parsers in `macos.go` take text, and their tests use real output in `testdata/`. macOS has no `cpu_busy_pct`, no `runqueue` and no `disk_busy_pct`. The design names `host_statistics` for the CPU, and that Mach call needs cgo. The design names no other CPU source for macOS, so the field is absent.
- **`compute`.** `cpu_busy_pct` is the change of busy ticks over the change of all ticks between two readings. `disk_busy_pct` is the change of `io_ticks` over the elapsed time, averaged over the disks and capped at 100. A counter that goes back gives 0.
- **`run` is the seam.** `main` passes the real source and a SIGTERM and SIGINT channel. Tests pass a fake reader and a fake stop channel. `-duration` ends a run without a signal.
- **A failed tick.** On macOS, a failed tool skips its field for that tick and the sampler goes on. On Linux, the loop prints the error on stderr and goes on. A failed first reading, an open error, a write error or a close error gives exit code 1.

## Invariants

- **The sampler writes at least one record each second and closes the file on SIGTERM.** `TestMain_TwoSecondRunWritesOneRecordPerSecondAndStopsOnSIGTERM` runs 2.2 s on the real host source and sends SIGTERM to its own process. It then checks the record count, the gaps and the exit code 0.
- **Each record is one atomic append.** The line and its newline go in one buffer, in one `Write` on an `O_APPEND` file. A reader, such as the artifact upload while the sampler still runs, sees only whole lines. Pinned by `TestRun_ConcurrentReaderSeesOnlyWholeLines`.
- **The sampler never fails a job.** On a platform with no source (`source_other.go`, any build that is not Linux and not macOS), `main` prints `unsupported platform` and exits 0. Pinned by `TestStart_UnsupportedPlatformLogsAndExitsZero`. The CI step builds and starts it in an `if`, and a failed build only prints a warning.
- **A bad host file is an error, not a zero.** Pinned by `TestReadLinux_MissingOrBadFilesFailLoudly` and `TestParseVMStat_BadInputFailsLoudly`.

## Findings

- The CI step keeps no pid file, because nothing stops the sampler.
- No file in this command needs cgo, so `CGO_ENABLED=0` builds all of it.
- If the design adds a macOS CPU source that needs no cgo, add it in `source_darwin.go` and set `HasCPU`.

Design: [ci-telemetry-and-critical-path §6](../ci-telemetry-and-critical-path.md#6-level-4-the-runner-sampler). Plan: component C0b of [ci-wall-time-2026-10](../../plans/ci-wall-time-2026-10.md).
