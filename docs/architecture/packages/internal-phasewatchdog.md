# internal/phasewatchdog

## Purpose

`internal/phasewatchdog` is the Go port of the shell script `phase-watchdog.sh`. It is an activity-based phase watchdog. A background loop polls the file modification times (mtimes) of a workspace. It kills a stalled process group when no file activity appeared for longer than the threshold.

## Design

- **The idle baseline is the later of two times.** One is the freshest mtime of the watched files. The other is the start time of the current phase.
- **A phase advance resets the clock to now.** Without the reset, a phase that writes nothing but still makes progress counts as stalled. The phase-advance tests in `phasewatchdog_test.go` pin this.
- **The watched files.** They are the four globs of the shell script in the workspace: `*.log`, `*.md`, `*.json` and `*-observer-events.ndjson`. They also include the cycle-state file. When `ProjectRoot` is set, they include `.evolve/ledger.jsonl`.
- **The idle clock starts when one watched file has an mtime.** `idleS` never goes below zero.
- **The warning comes before the stall.** It fires at `WarnPct` percent of the threshold (default 75), once for each idle stretch. New activity arms it again.
- **Defaults.** `ThresholdS` is 600, `PollS` is 15, `WarnPct` is 75 and `GraceS` is 10. `Disabled` carries `ObserverPolicy.WatchdogDisabled`.
- **Exit codes.** `ExitOK` (0) means the kill sequence completed or the watchdog is disabled. `ExitInvalidArg` (1) means invalid arguments or a missing workspace.
- **The test seams are `Now`, `Sleep`, `KillPgrp` and `StopAfter`.** `StopAfter` stops the loop after N iterations (0 means unlimited). A test reaches a kill only through `KillPgrp`.
- **`fireSequence` has a fixed order.** It writes `stall-progress.json` through a temp file and a rename. It appends the abnormal event. It sends `SIGTERM` to the target process group, waits `GraceS`, and then sends `SIGKILL`.
- **The abnormal event is best effort.** `appendAbnormalEvent` ignores errors.
- **`defaultKillPgrp` sends the signal to `-pgid`.** That signals the whole process group.

## Invariants

- **The kill reaches only the process group that the caller named.** `Run` rejects a `TargetPGID` that is not positive. The open inbox item `kill-pgid-sites-injected-sender` covers a stronger guard for the sites that call `syscall.Kill(-pgid, ...)`.
- **Until that guard exists, a test must not run the real `defaultKillPgrp` with a signal other than 0.** A test must also not use a pgid that it did not create. `TestDefaultKillPgrp_Signal0Succeeds` uses signal 0, the POSIX existence probe, which kills nothing.
- **The abnormal event uses the shared vocabulary.** `abnormal-events.jsonl` has two writers: this package and `internal/dispatchevents`. The severity must be in the enum `WARN|ERROR`.
- **The record once used the severity `HIGH`.** It parsed into the field, and every filter keyed on `WARN|ERROR` dropped it. The event type must be a type that `dispatchevents` declares (`EventStallDetected`). Then a typed reader can switch on it.
- **A record that falls outside the vocabulary is the worst kind of telemetry defect.** A stall that killed a phase becomes invisible to the queries of the operator. `abnormal_vocabulary_test.go` pins this. It decodes the watchdog record through the type of the dispatcher.
