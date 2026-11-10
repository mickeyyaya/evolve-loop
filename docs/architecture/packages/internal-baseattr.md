# internal/baseattr

## Purpose

`internal/baseattr` says if a red test belongs to the lane or to its base. The design is in [gate-contract-g4-base-red-attribution.md](../gate-contract-g4-base-red-attribution.md).

## Design

- `ParseGoTest` parses the `--- FAIL:` and `FAIL` package lines. It becomes the one parser when the build floor takes its keys from it. A key is a package and a top-level test. A subtest maps to its top-level test.
- A panic, a `FAIL` line with no named test and a test name with no package line set `Unnamed`. Then `Attributable` is false, and `Attribute` returns a verdict with `Unjudged` set and no run.
- `Attribute` reruns only the red keys on the lane base, in a detached worktree that has a `.git` entry. It splits the keys into `LaneOnly` and `BaseRed`.
- When `BaseRed` is not empty, `Attribute` reruns those keys on the `main` ref. It skips that run when `main` is the base.
- `RunAt` adds the worktree under `Scratch` through `gitexec`, runs the `Runner` in the Go module, and removes the worktree in a `defer`.
- `GoTestRunner` runs `go test` through the `sysexec` seam with the environment of the caller. The remaining deadline of the context becomes the `-timeout` flag.
- `Verdict.Blamed` is true when `LaneOnly` is not empty or `Unjudged` is set. `Verdict.MainGreen` is true when the base is red and `main` is read and green.

## Invariants

- A run that fails is an error and never an empty result. The caller then blames the lane and signals.
- An unreadable `main` ref returns the base-red keys with an empty `Main` and no error.
- The package imports `gitexec`, `sysexec`, `lanebudget` and the standard library. It imports no pipeline package.
- The path is on the protected surface as a gate guarantee. A lane cannot edit it.
- The policy value `gates.base_red_rerun_seconds` sets the time limit of a rerun. The default is 300 seconds.
