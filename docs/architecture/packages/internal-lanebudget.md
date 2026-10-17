# internal/lanebudget

> The spec: "The lane CPU budget" in §12 of [wave-orchestrator](../wave-orchestrator.md). The plan: component O9 of [the wave orchestrator plan](../../plans/wave-orchestrator-2026-10.md). The width key: [internal-ipcenv](internal-ipcenv.md).

## Purpose

`internal/lanebudget` gives each `go test` that a fleet lane runs one share of the host CPUs. The share is `max(1, host CPUs / fleet width)`. Without it, each lane's `go test` uses all the CPUs of the host. Then 4 lanes put a load of 15–18 on 14 CPUs (spec §12, F12.1). A console run has no fleet width, so the package changes nothing there.

## Design

- **`Procs(hostCPUs, width)`** is the formula. A width below 1 means "no lane" and gives 0. Otherwise the result is the CPU count divided by the width, with a floor of 1. Example: `Procs(14, 4)` is 3, and `Procs(2, 8)` is 1.
- **`Current()`** reads `EVOLVE_FLEET_WIDTH` (`ipcenv.FleetWidthKey`) from the environment of this process and the CPU count from `runtime.NumCPU`. Each lane is a separate process (`execCycleLaunch` in `cmd/evolve/cmd_fleet.go`), so the width that it reads is its own. An absent or garbage width gives 0.
- **`Args(args)`** adds `-p N` and `-parallel N` after the `test` word of a `go test` argv. It keeps a `-p` or `-parallel` that the caller already set. It does not change an argv that does not start with `test`, and it does not write into the array of the caller.
- **`Env(env)`** adds `GOMAXPROCS=N` at the end of the child environment, so it wins over an inherited value. A nil `env` means "inherit", as in `exec.Cmd`, so `Env(nil)` starts from `os.Environ()`. Give it the environment after `ipcenv.Scrub`: the scrub removes the `EVOLVE_` keys but keeps `GOMAXPROCS`.
- **`Runner(run)`** wraps a `sysexec.RunFunc`. It applies `Args` and `Env` to each `go test` call and passes every other call through. Put it inside a wrapper that replaces the environment (`ciparitygate.scrubbedRun`), so that the replaced environment still gets `GOMAXPROCS`.
- **Why all three values.** `-p` bounds the packages that run at the same time, and `-parallel` bounds the parallel tests in one binary. They multiply. Only `GOMAXPROCS` bounds the compiler, the linker and the CPU use inside each test binary.

## The call sites

| Site | How it goes through the budget |
|---|---|
| `phases/ship/repocontract.go` (`repoContractTestArgs`, `runGoTestJSON`) | `Args` and `Env`. The importer backstop of `repocontract_importers.go` runs through the same two functions. |
| `acssuite/acssuite.go` (`defaultGoExec`) | `Args` and `Env` |
| `acsrunner/runner.go` (`Run`, `execCommand`) | `Args` and `Env` |
| `core/task_contract.go` (`go test -list`) | `Args` and `Env`: the list run still compiles the package |
| `core/build_floor_reviewer.go` (`coverTestArgs`, `scopedCoverProfile`) | `Args` and `Env` |
| `core/phase_bindings_selfcheck.go` (`unitTestArgs`, `taggedTestArgs` and their runners) | `Args` and `Env` |
| `phases/audit/ciparitygate/tier.go` | `Current` replaces the fixed 4 in a lane, and `Runner` adds `GOMAXPROCS` |
| `phases/audit/ciparitygate/command.go` (acs-durable) and `apicover.go` | `Runner` |
| `ciparity/ciparity.go` (`CoverageTestArgs`) | `Args` |
| `commitgate/lanes.go` (the Go lane) | `Runner`, through `Options.budgeted` |

`releasepreflight/releasepreflight.go` is exempt: it runs only on the console, before a release. `gotooltest/cache.go` runs `go build`, not `go test`, and only from test code, which already runs under the budget of its parent.

## Invariants

- The formula and the two "no lane" answers: `TestProcs_DividesTheHostAmongTheLanes`, `TestCurrent_ReadsTheFleetWidthOfThisProcess`.
- A console run gets no flag and no `GOMAXPROCS`: `TestArgs_AConsoleRunIsUnchanged`, `TestEnv_AConsoleRunIsUnchanged`.
- Each non-test file under `go/internal` or `go/cmd` that builds a `go test` argv imports this package: `TestEveryGoTestArgvGoesThroughTheLaneBudget` in `go/test/structure`. The test finds a `[]string{"test", …}` literal and a call with the arguments `"go", "test"`. It does not prove that each argv in the file goes through the budget, only that the file uses it. Each site has its own test that runs it in a lane.
