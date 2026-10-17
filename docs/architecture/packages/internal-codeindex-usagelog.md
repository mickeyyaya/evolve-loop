# internal/codeindex/usagelog

> The spec: [code-index-cli](../code-index-cli.md), section 6. The plan: component K3 of [the code index plan](../../plans/code-index-cli-2026-10.md). The operator surface: `evolve code usage` in [runtime-reference.md](../../operations/runtime-reference.md).

## Purpose

`usagelog` writes and reads the usage log of the `evolve code` commands. The log is the measure for goals G1, G2 and G3 of the plan: it shows which phase and which CLI called which command.

## Design

- **One record for each call.** `Record` holds the time, the command and the argument length. It also holds the exit code, the hit count and the duration. The last fields are the phase, the cycle, the CLI family and the working directory. It never holds the argument. The argument can hold code.
- **Whole lines only.** `Append` encodes the record, adds a newline and writes it with one `write` call on a file that it opens with `O_APPEND`. A reader never sees a part of a line. Two calls at the same time never mix their lines.
- **The caller owns the failure.** `Append` returns the error. The command prints one line on stderr and keeps its exit code and output.
- **Identity from the environment.** `Identity` reads the phase and the cycle from `EVOLVE_DISPATCH_ID` and the CLI family from `EVOLVE_CLI`. It takes a lookup function, so a test needs no process environment.
- **A tolerant reader.** `Read` returns the records at or after a start time. A missing file gives an empty result. A line that does not parse is skipped and counted.
- **A summary.** `Summarize` counts the calls, the hits and the failures for each phase and command, in sorted order.

## Invariants

- **Path rule.** `Target` gives the log path. Inside a phase (`EVOLVE_DISPATCH_ID` is set), the path is `<project root>/.evolve/runs/cycle-N/code-calls.ndjson`. Outside a phase, the path is `<user cache dir>/evolve/code-calls.ndjson` (`UserPath`).
- **Reason.** The Linux phase sandbox mounts a private `tmpfs` on `/tmp` (`--tmpfs /tmp`), so a record in `/tmp` vanishes when the phase ends. The cycle workspace is writable under both sandboxes.
- **Modes.** `Append` creates the directory with mode 0700 and the file with mode 0600.
- **Rotation.** `Append` takes a limit. When the file is larger than the limit, `Append` renames it to `<file>.1` before the write and keeps one old file. `Target` gives `MaxUserBytes` (5 MB) for the per-user file and 0 (no rotation) for a workspace file. The existing gc removes a workspace file with the run directory of the cycle.
- **Reading.** `ReadAll` merges the per-user file, its `.1` file and every cycle workspace file of the project.
- The package is held at 100% statement coverage in `go/.cover-strict` and is enrolled in `go/.apicover-enforce`.
