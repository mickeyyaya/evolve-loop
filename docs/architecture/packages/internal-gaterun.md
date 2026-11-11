# internal/gaterun

> The design: section 7.16 (R1, R2 and R6) of [gate-contract-g2-hermetic-guard](../gate-contract-g2-hermetic-guard.md), parts (a) and (b) (ADR-0165).

## Purpose

`internal/gaterun` runs a gate entry in a child process. The child reads no git and starts no toolchain. Each such read goes to the parent through a broker.

## Design

- **`Run`.** It starts the child with `Options.Command`, sends the `input` frame and serves each `request` frame. `Options.Strategy` is the one seam that wraps the child. `Run` asks it for the prefix of the run. A nil strategy or a nil prefix leaves the command as it is.
  - Before the child starts, `Run` puts the two canary files in the scratch dir.
  - After the child ends, `Run` calls the `Finish` of the strategy when it has one (`Finisher`).
- **`EvolveCommand`.** It builds `evolve gate run --entry <name>` through `sysexec.Command`. A test injects its own command.
- **`Serve`.** It is the child side. It reads the `input` frame, calls `entry.Run` with `Ports` that are broker clients, and writes the `verdict` frame. A failure writes an `error` frame.
- **Frames.** Each frame is one JSON line with a `kind`: `input`, `request`, `response`, `verdict` or `error`. `ReadFrame` refuses an unknown kind and a frame that lacks the field of its kind.
- **`Broker`.** It serves the five functions of `Lane` and `Toolchain`. A request for another function gets a `response` frame with a message, and the call of the child returns that error.
- **The boundary.** The broker pins each `Lane` root and each `Call.Dir` to the `Input.Root` of the run. A `RelDir` that is absolute or holds a parent element is refused, and the broker drops the `Env` of the child. The go run takes the environment of the parent with `GOPROXY=off`, `GOFLAGS=-mod=readonly` and `GOTOOLCHAIN=local`, so no request reaches the network or another toolchain. Each verb has a closed list of flags in `policy.go`. The broker refuses these before any process starts:
  - a flag outside the list, or a flag that is missing its value;
  - a package argument other than `.` or a path that starts with `./`;
  - a package argument that holds `@` or a parent element, for example `rsc.io/quote@latest`;
  - a `-coverprofile` outside the scratch directory of the run.
- **Bounded frames.** `ReadFrame` refuses a line over `MaxFrameBytes` (4 MiB). `Run` makes a scratch directory for each run (under `Options.ScratchParent`) and removes it at the end.
- **`RealPorts`.** It wraps `repostate` as `Lane` and `gotool` as `Toolchain`. A `Toolchain` call returns an `Output` with the exit code. A non-zero exit is not an error of the call. Only a failure to start is an error.

## The mode (part (b), R4)

`NewStrategy(Setup)` reads `gates.hermetic.mode` (`policy.Policy.HermeticMode`, default `shadow`) and the OS, and returns one strategy:

| Mode | macOS | Linux |
|---|---|---|
| `off` | no prefix | no prefix |
| `shadow` | `sandbox-exec` with the report profile. Each deny is an `allow` with a report. After the run, `Finish` reads the reports and emits `GATE_HERMETIC_VIOLATION` (entry, operation, path). | no prefix. `Finish` emits one `GATE_HERMETIC_NO_EVIDENCE`, because `bwrap` cannot report. |
| `enforce` | `sandbox-exec` with the deny profile. `Finish` reads no reports, because the deny profile tags none. | `bwrap` (`/usr/bin/bwrap`) with only the `evolve` binary and its libraries in the root |

- Each strategy wraps the command in place (`Path`, `Args`, `Env`), so the context, the cancel function and `WaitDelay` of `sysexec.Command` stay. The wrapper sets `TMPDIR` to the scratch dir.
- The tag of a run is the name of its scratch dir. The report profile and the report query share it.
- `Serve` reads the start canary before the entry and the end canary after it, from `TMPDIR`. With no wrapper (mode `off`), the reads find no file and change nothing.
- At shadow, `Finish` emits `GATE_HERMETIC_NO_EVIDENCE` with the cause in each of these cases, and never passes silently:
  - the reports cannot be read;
  - the fence of the run does not show, so the read is incomplete;
  - a canary report is missing.
- The last case is real. On 2026-11-11 the kernel logged no report of a newly built binary while AMFI checked its ad-hoc signature. The fence still showed, because `cat` (a platform binary) wrote it. Only a report of the child itself proves that its reports reached the log.
- A mode other than `off` with no `Signals` is an error of `NewStrategy`, because the evidence is then lost. An OS with no sandbox is an error too.
- Linux `enforce` reads the libraries of the binary with `/usr/bin/ldd` in `NewStrategy`, and a failure of `ldd` is an error there. The fixed paths keep a planted `bwrap` or `ldd` on `PATH` out of the run.
- `SetupFor` builds a `Setup` from the policy. The caller sets `Signals`.

## The child verb (part (b), R1)

`evolve gate run --entry <name>` looks the entry up in `gateregistry.Entries()` and calls `Serve`. An unknown entry is a usage error that lists the known names. Only the parent of a run starts the verb, so it is a pipeline tool and not a core verb ([cli-contract.md](../cli-contract.md) §5). An agent in a cell gets class `D`, because the agent-class allowlist does not name it.

## Signals

`GATE_HERMETIC_VIOLATION` and `GATE_HERMETIC_NO_EVIDENCE` use the module `gate.hermetic`.

## Invariants

- An `error` frame, a non-zero exit, a malformed frame and a stream with no `verdict` each give an error and no `Verdict`. None gives a pass.
- The child stderr text is part of the error.
- The `Verdict` of `entry.Run` crosses the frames unchanged.

## Findings

- The module `gate.hermetic` is `signalcenter.ModuleGateHermetic`, so a real `Center` keeps the two codes.
- At `shadow` the macOS profile reports the default deny first (rule `gate.default`). The last rule that matches wins, so each grant after it gives no report. On a host run, `git log` read the CommandLineTools SDK, `~/.gitconfig` and the `.git` common dir. A static list of reads finds none of these.
- The child reads the node of the directory of its own binary. Enforce refused that read and the child went on, so the profile grants the node (a `literal`, not a `subpath`). Without the grant, each clean shadow run gave one false report.
- The canaries bracket the run. They detect a loss of all reports, as in the AMFI case. They do not detect a loss in the middle of the run.
- `ParseReports` keeps at most `ReportCap` reports of each rule and drops the rest with no signal.
- The profile allows `file-read-metadata` everywhere, so an entry can stat a path outside the grants with no report.
- An inbox item (`gate-hermetic-round-two-followups`) tracks these limits and the open round-2 items. Examples are one darwin strategy type for each mode and a kind for `NO_EVIDENCE`.
- `Toolchain` returns an `Output`, not an `*exec.Cmd`. A command cannot cross the frame channel, so the parent runs it and sends the captured result.
- The child gets the full environment of the parent. At shadow the network is open with a report, so an entry can send a variable out before the report is read. An inbox item tracks an allowlist of the environment of the child.
- The sandbox cannot run in a nested session. The integration tests (`-tags integration`) skip there. Run them on a top-level macOS host. On 2026-11-11 they passed 8/8 (two rounds of four tests) on such a host.
