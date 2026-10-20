# Build Explanation — Cycle 1978

## Build Binding
- Cycle: 1978
- Base SHA: e66b2a66792670e650922b3f637176446143d611

## Summary
The named projection `ipcenv.GoTestEnv` exists and the production go test spawners outside the protected control-plane paths build their env through it or through `ciparity.CIEnv`. A whole-tree AST test in `internal/ipcenv` fails when a spawner builds its env any other way. The exemption table records, with a reason, `worktreeGenerator` and `ExecRunner` (they do not run `go test`), `ciparitygate.runAttempt` (its env arrives pre-built), and six spawners on protected control-plane paths (ADR-0064) that a lane cannot edit: `acssuite.defaultGoExec`, `commitgate.laneGo`, `core.scopedCoverProfile`, `core.realGoUnitTest`, `core.realGoUnitTestTagged` and `ciparitygate.ACSDurable`. Converting them is console work.

## Rationale
A spawner that inherits the lane's `EVOLVE_FLEET` and cycle-state keys makes env-sensitive packages go false-RED (the 2026-09-14 and cycle-1700 class). Each spawner had its own `ipcenv.Scrub(os.Environ())` call or none at all: `pkg/acsassert`, the commit-gate go lane and the ACS-durable gate inherited the process env. One projection removes the per-site choice, and the AST pin makes the rule a gate instead of a convention.

## Changed Areas
- `go/internal/ipcenv/scrub.go` — adds `GoTestEnv(extra ...string)`: the scrubbed ambient env with the extras last.
- `go/internal/sysexec/sysexec.go` — adds `WithEnv`, a `RunFunc` wrapper that pins the child env; three call sites needed the same closure.
- `go/pkg/acsassert/go_tests.go` — `GoTests` runs its child `go test` through `WithEnv(…, GoTestEnv())`; it inherited the caller's env before.
- `go/internal/acsrunner/runner.go` — the commander receives the env from `Run`, which builds it with `GoTestEnv`.
- `go/internal/core/task_contract.go` — the predicate listing `go test -list` uses `GoTestEnv`.
- `go/internal/phases/ship/repocontract.go` — the repo-contract `go test -json` uses `GoTestEnv`.
- `go/internal/releasepreflight/releasepreflight.go` — `goTestCommand` uses `GoTestEnv`.
- `go/internal/repocontract/repocontract.go` — `internal/ipcenv` joins `Packages()`, because the pin reads the whole tree.
- `go/internal/ipcenv/gotestenv_pin_test.go` — the pin and its tests; the exemption table gains the entries above with reasons.
- `go/acs/cycle1978/predicates_test.go` — the ACS predicates of the cycle.
- `.evolve/evals/go-test-env-projection-pinned-lane.md` — the eval of the task.
- `go/pkg/acsassert/go_tests_env_integration_test.go` — proves the child `go test` of `GoTests` sees no `EVOLVE_` key of its caller.
- `go/internal/acsrunner/fail_closed_test.go` — the env assertions move from `execCommand` to the env that `Run` hands to the commander.
- `go/internal/acsrunner/lanebudget_test.go` — the same move for the CPU-budget assertion; commander stubs take the new `env` argument.
- `go/internal/acsrunner/runner_test.go` — commander stubs take the new `env` argument.
- `go/internal/phases/ship/repocontract_pack_test.go` — the expected red message names the added `ipcenv` suite.
- `go/internal/ipcenv/scrub_test.go` — names `GoTestEnv` for the repo-wide apicover gate.
- `go/internal/sysexec/sysexec_cover_test.go` — names `WithEnv` for the repo-wide apicover gate.

## Design Decisions
The pin accepts a spawner whose body calls the projection, or calls a function whose body does (`scrubbedRun`, `goTestOptions`). Rejected: a shared `RunFunc` in `ipcenv`, because `ipcenv` would import `sysexec`. Rejected: keep `execCommand` signature in `acsrunner` and project inside it, because the pin reads `Run`, the spawner, not the generic commander. `runAttempt` stays unconverted and exempt: wrapping it again would capture the env twice, and the retake must carry the env of the first attempt (`TestTierAttempts_ScrubbedEnvIsCapturedOnceAndIdenticalOnBothAttempts`).

## Verification
With the lane keys unset, the touched packages pass, the pin included, and the cycle ACS package passes. Each new gate test was run red against the old code first. The results are in `build-report.md`.

## Compatibility
No exported symbol changes except the two additions. `acsrunner`'s unexported commander type gains an `env` parameter. The commit gate, the build floor, the phase self-check, the ACS suite and the ACS-durable gate keep their old env until the console work lands.

## Limitations
Six protected-path spawners and `runAttempt` are exempt, not converted. Acceptance 2 is met for every spawner a lane may edit; the protected ones need the console.
