# 2026-11-14: the composed-tree gates loaded the host to 75 at four lanes

The operator asked: "investigate why CPU loading is so high on 4 lanes, it doesn't make sense."

Related documents: the package note [internal-lanebudget.md](../architecture/packages/internal-lanebudget.md), the decision [ADR-0166](../architecture/adr/0166-composed-gates-test-only-the-affected-packages.md) and the design [composed-gate-scope.md](../architecture/composed-gate-scope.md).

## Summary

- **Impact:** wave 171 ran four lanes on a host with 14 CPUs. The load average was 50 to 75. The CPU was 46% user, 54% system and 0% idle, with about 5000 disk IO operations each second.
- **Root cause:** the composed-tree gate ran `make` with no CPU limit. The go command then used 14 as `-p` and as `-test.parallel`. The lane budget bounded only the `go test` calls that go through the runner.
- **Effect:** in a 30 s sample, the `make` trees of the composed gates used 248% CPU. The macOS services used 78%, the budgeted lane floors 33% and the agents 13%.
- **Fix:** pull request #1448 gives the lane budget to the `make` trees. A structure guard finds each `make` run that skips the budget. The `test` target passes `-timeout $(TEST_TIMEOUT)`.
- **Follow-up:** [ADR-0166](../architecture/adr/0166-composed-gates-test-only-the-affected-packages.md) makes the composed gates test only the affected packages.

## Terms used throughout

- **A lane:** one parallel worktree of a wave.
- **The lane budget:** the package `internal/lanebudget`. It divides the host CPUs among the lanes.
- **A composed-tree gate:** a `make` target that the runner runs on the merged tree: `build`, `test`, `test-acs-durable` and `apicover-enforce`.
- **The load average:** the count of runnable processes on the host, averaged over time.

## Timeline

| Time | Event | Evidence |
|---|---|---|
| 2026-11-14 | Wave 171 runs four lanes. The load average is 50 to 75 and no CPU is idle. | measurement of the console, 14 CPUs |
| 2026-11-14 | The 30 s attribution names the `make` trees of the composed gates as the largest user. | 248% CPU against 33% for the budgeted lane floors |
| 2026-11-14 13:08 (+0800) | Pull request #1448 lands. | commit `9c5aafd5a` |
| 2026-11-14 21:52 (+0800) | ADR-0166 lands and limits the scope of the composed gates. | commit `7603c3e52` |

## Root cause

**F1.1** `composedGateRun.gate` runs `make -C go <target>` for the targets `build`, `test`, `test-acs-durable` and `apicover-enforce` (`go/cmd/evolve/cmd_composition_wiring.go:49-53`, `:133`). Before #1448 it set `cmd.Env = ciparity.CIEnv(os.Environ())` and no `GOMAXPROCS` (`git show 9c5aafd5a -- go/cmd/evolve/cmd_composition_wiring.go`).
*Implication:* the `make` tree has no CPU limit.

**F1.2** The go command takes its default `-p` from `GOMAXPROCS`, and `-test.parallel` follows the same value (package note `internal-lanebudget.md`). On this host the value was 14.
*Implication:* each gate tree started up to 14 build jobs and 14 parallel tests.

**F1.3** The `test` target runs the whole module with `-race` (`go/Makefile:110`).
*Implication:* one gate run costs the CPU of the full race suite, also for a change in one package.

**F1.4** The lane budget changed only the `go test` argv that goes through `Runner` and `Args` (package note `internal-lanebudget.md`). A `make` recipe that runs `go test` was outside that path.
*Implication:* the budget held for the lane floors (33%) and not for the gates (248%).

**F1.5** Four lanes ran their gates at the same time on one host.
*Implication:* the sum of four unbounded trees gave a load average of 50 to 75 on 14 CPUs.

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Set `cmd.Env = lanebudget.Env(ciparity.CIEnv(os.Environ()))` in `composedGateRun.gate`. | F1.1, F1.2 | Add `-p` flags to each recipe: `GOMAXPROCS` alone sets both `-p` and `-test.parallel`, in one place. |
| D2 | Add the structure guard `TestEveryMakeRunGoesThroughTheLaneBudget`. | F1.4 | Fix only the one call: the next new `make` run brings the load back. |
| D3 | Make the `test` target pass `-timeout $(TEST_TIMEOUT)`. | F2.1 | Keep the default timeout of 10 minutes: the slowest package then fails. |

**F2.1** The console measured the cost on 2026-11-14, on a host with 14 CPUs and a live wave. The race suite of `make test` took 984 s at `GOMAXPROCS=3`. Its slowest package, `cmd/evolve`, took 568 s. The default timeout of one test binary is 10 minutes (package note `internal-lanebudget.md`).
*Implication:* a lane share makes each package slower, and the default timeout is close to the limit. `TEST_TIMEOUT` is 20 minutes (`go/Makefile:117`).

## What changed

| Change | Regression test (red without the change) |
|---|---|
| D1 in `go/cmd/evolve/cmd_composition_wiring.go` (#1448, commit `9c5aafd5a`) | `TestRunComposedGates_ALaneGateGetsItsCPUShare`, `TestRunComposedGates_AConsoleGateGetsNoCPULimit` |
| D2 in `go/test/structure/lanebudget_guard_test.go` (#1448) | `TestEveryMakeRunGoesThroughTheLaneBudget` |
| D3 in `go/Makefile` (#1448) | none: the measurement F2.1 gives the reason |

The console runs get no limit, as before. The `make build` calls in `cmd_wave.go` and `cmd_loop_boot_refresh.go` stay exempt, because they run on the console and never in a lane.

ADR-0166 then cut the work of each gate. The runner passes the affected packages to `make test` as `FAST_PKGS` and to `make apicover-enforce` as `APICOVER_PKGS`.

## Limits

- The lane share is a bound on the tree, not a guarantee. Four lanes that run `test-acs-durable` together still use the share of four lanes.
- The guard finds a call that has the literal `"make"` as its first or second argument. A `make` name that a variable holds escapes it.

## What we learned

- A budget must cover each path that starts work, not only the path that the first author saw. The guard `TestEveryMakeRunGoesThroughTheLaneBudget` now fails when a function runs `make` without `lanebudget.Env`.
- A lane share makes a suite slower. A timeout must match the measured time at the share, not the time at full speed. The `test` target now uses `TEST_TIMEOUT`.
- Before you raise the lane count, confirm that the fix is live and that the load is under the CPU count.
