# Convention: driver tests must be environment-hermetic

**Status:** active · **Introduced:** cycle-1256 · **Scope:** `go/internal/bridge` (and each package whose production path branches on an ambient env read)

## The rule

A test that constructs a bridge `Engine` MUST pin `Deps.LookupEnv`. In
`go/internal/bridge`, construct the engine through `newTestEngine`
(`launch_test.go`). When a test does not supply `Deps.LookupEnv`, `newTestEngine`
sets it to the empty lookup. Call `NewEngine` directly only when the test is
*about* env resolution and pins the environment itself.

A test whose purpose is to exercise an env branch opts **in**. It uses an explicit
`Deps.Env` entry (`lookupEnv` reads it first) or its own `Deps.LookupEnv`.
It never inherits the values that the parent process exports.

## Why

`lookupEnv` (`driver_common.go`) resolves `Deps.Env` → `Deps.LookupEnv` →
`os.LookupEnv`. That last hop is a defensive fallback for production. In a test,
it is a hole to the ambient process environment.

`runTmuxREPL` reads `ipcenv.FleetKey` (`EVOLVE_FLEET`) through that same
`lookupEnv`. Under a fleet supervisor with no `--worktree`, the CB.2 guard
correctly fails closed with `errWorktreeRequired` → `ExitBadFlags` (10), *before*
the artifact wait loop runs. With `Deps.LookupEnv` nil, that read reached the
ambient env. Thus **20 tests** in the package passed in a developer shell and
failed under the ACS/EGPS gate. The gate shells `go test` with a bare
`exec.CommandContext` and no `cmd.Env` (`internal/acsrunner/runner.go`).
Thus it inherits the `EVOLVE_FLEET=1` of the orchestrator (`internal/fleet`).

Only one of the 20 tests became a RED at the gate, because an ACS predicate runs
a *named* subset. The predicates never selected the other 19. That single red
cost cycle-1252 and cycle-1254 a run each.

## Two things this convention deliberately does not do

- **It does not weaken CB.2.** The fleet guard behaves correctly. The
  fixtures were porous.
- **It does not sanitize `EVOLVE_*` at a consumer.** The `sanitizeEnv` of `internal/core`
  is exactly that workaround, and its own comment names this
  exit-10 failure. But the defect survived and hit two later cycles, because a
  workaround at one caller does not travel to the next runner that someone builds.
  To sanitize hides an env-porous test. It does not make the test hermetic. Fix the
  fixture.

## Regression guard

`TestRunTmuxREPL_ArtifactDebounceHermeticUnderAmbientFleetEnv`
(`completion_debounce_test.go`) exports `EVOLVE_FLEET=1` into the process. It
asserts that the artifact caller-proof still reaches the wait loop and exits
`ExitArtifactTimeout`. If you put an ambient env read back in these fixtures, the test
fails with exit 10 and the refusal of the driver on stderr.

## Verification method (the durable part)

When a gate red is "environment dependent", the falsification set MUST include
**the ambient environment itself**. Diff `env` between the agent shell and the
gate subprocess, and run again under the env of the gate. Do this *before* the load, PATH or
worktree-identity hypotheses. Grep the code path that fails for `os.Getenv` /
`os.LookupEnv` / `lookupEnv` branches, and toggle each.

`evolve selfcheck build` GREEN is **not** evidence against an env-porosity
hypothesis. It sanitizes `EVOLVE_*` by design, and it structurally cannot
see this class.

Source: `.evolve/instincts/lessons/cycle-1254-driver-tests-inherit-ambient-fleet-env-gate-only-red.yaml`
