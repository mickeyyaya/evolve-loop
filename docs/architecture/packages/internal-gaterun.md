# internal/gaterun

> The design: section 7.16 (R1, R2 and R6) of [gate-contract-g2-hermetic-guard](../gate-contract-g2-hermetic-guard.md), part (a) (ADR-0165).

## Purpose

`internal/gaterun` runs a gate entry in a child process. The child reads no git and starts no toolchain. Each such read goes to the parent through a broker.

## Design

- **`Run`.** It starts the child with `Options.Command`, wraps it with `Options.Prefix`, sends the `input` frame and serves each `request` frame. A nil `Prefix` leaves the command as it is. Part (b) gives the sandbox prefix.
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

## Invariants

- An `error` frame, a non-zero exit, a malformed frame and a stream with no `verdict` each give an error and no `Verdict`. None gives a pass.
- The child stderr text is part of the error.
- The `Verdict` of `entry.Run` crosses the frames unchanged.

## Findings

- `Toolchain` returns an `Output`, not an `*exec.Cmd`. A command cannot cross the frame channel, so the parent runs it and sends the captured result.
- The `evolve gate run` verb and the manifest row `/go/internal/gaterun/` are not in this part.
