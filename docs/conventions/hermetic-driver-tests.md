# Convention: driver tests must be environment-hermetic

**Status:** active · **Introduced:** cycle-1256 · **Scope:** `go/internal/bridge` (and any package whose production path branches on an ambient env read)

## The rule

A test that constructs a bridge `Engine` MUST pin `Deps.LookupEnv`. In
`go/internal/bridge` this is done by constructing through `newTestEngine`
(`launch_test.go`), which defaults `Deps.LookupEnv` to the empty lookup when a
test does not supply one. Call `NewEngine` directly only when the test is
*about* env resolution and pins the environment itself.

A test that means to exercise an env branch opts **in** — via an explicit
`Deps.Env` entry (consulted first by `lookupEnv`) or its own `Deps.LookupEnv` —
never by inheriting whatever the parent process happens to export.

## Why

`lookupEnv` (`driver_common.go`) resolves `Deps.Env` → `Deps.LookupEnv` →
`os.LookupEnv`. That last hop is a defensive fallback for production; in a test
it is a hole to the ambient process environment.

`runTmuxREPL` reads `ipcenv.FleetKey` (`EVOLVE_FLEET`) through that same
`lookupEnv`. Under a fleet supervisor with no `--worktree`, the CB.2 guard
correctly fails closed with `errWorktreeRequired` → `ExitBadFlags` (10) *before*
the artifact wait loop runs. With `Deps.LookupEnv` nil, that read reached the
ambient env — so **20 tests** in the package passed in a developer shell and
failed under the ACS/EGPS gate, which shells `go test` with a bare
`exec.CommandContext` and no `cmd.Env` (`internal/acsrunner/runner.go`) and
therefore inherits the orchestrator's `EVOLVE_FLEET=1` (`internal/fleet`).

Only one of the 20 ever surfaced as a gating RED, because an ACS predicate runs
a *named* subset — so the other 19 were simply never selected. That single red
cost cycle-1252 and cycle-1254 a run each.

## Two things this convention deliberately does not do

- **It does not weaken CB.2.** The fleet guard is behaving correctly. The
  fixtures were porous.
- **It does not sanitize `EVOLVE_*` at a consumer.** `internal/core`'s
  `sanitizeEnv` is exactly that workaround, and its own comment names this
  exit-10 failure — yet the defect survived to bite two later cycles, because a
  workaround planted at one caller does not travel to the next runner built.
  Sanitizing hides an env-porous test; it does not make it hermetic. Fix the
  fixture.

## Regression guard

`TestRunTmuxREPL_ArtifactDebounceHermeticUnderAmbientFleetEnv`
(`completion_debounce_test.go`) exports `EVOLVE_FLEET=1` into the process and
asserts the artifact caller-proof still reaches the wait loop and exits
`ExitArtifactTimeout`. Reintroduce an ambient env read in these fixtures and it
fails with exit 10 and the driver's own refusal on stderr.

## Verification method (the durable part)

When a gate red is "environment dependent", the falsification set MUST include
**the ambient environment itself** — diff `env` between the agent shell and the
gate subprocess and re-run under the gate's env — *before* load, PATH or
worktree-identity hypotheses. Grep the failing code path for `os.Getenv` /
`os.LookupEnv` / `lookupEnv` branches and toggle each.

`evolve selfcheck build` GREEN is **not** evidence against an env-porosity
hypothesis: it sanitizes `EVOLVE_*` by design and is structurally incapable of
seeing this class.

Source: `.evolve/instincts/lessons/cycle-1254-driver-tests-inherit-ambient-fleet-env-gate-only-red.yaml`
