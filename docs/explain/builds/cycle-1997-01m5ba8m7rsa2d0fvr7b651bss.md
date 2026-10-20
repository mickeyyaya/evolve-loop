# Build Explanation — Cycle 1997

## Build Binding
- Cycle: 1997
- Base SHA: ca763e7c23214c70449e0e704ce09e116057f4b2

## Summary
Every production go test spawner now builds its child env from one projection, ipcenv.GoTestEnv, and a whole-tree AST pin fails any spawner that does not use that projection or ciparity.CIEnv. The pin also rejects a spawner that calls a projection but still passes the ambient process env.

## Rationale
Spawners built their env in different ways: acsrunner, core, ship and releasepreflight each wrote Scrub(os.Environ()), and pkg/acsassert ran its child go test on the caller's full env, so EVOLVE_ keys of the caller reached the child. One named projection, ipcenv.GoTestEnv, gives all of them the same env, and the pin keeps it that way. The first pin counted a spawner as projected when any projection call appeared in its body, so a decoy call, or an append of os.Environ() onto the projection, passed. The pin now also requires that no os.Environ() call sits outside a projection call argument.

## Changed Areas
- `go/internal/ipcenv/gotestenv_pin_test.go` — the whole-tree AST pin; this cycle adds projectsWithoutAmbientEnv and two planted cases (decoy projection, ambient env appended back).
- `go/internal/ipcenv/scrub.go` — carries the GoTestEnv projection salvaged from cycle 1978.
- `go/internal/ipcenv/scrub_test.go` — tests for GoTestEnv.
- `go/pkg/acsassert/go_tests.go` — runs its child go test on the GoTestEnv projection.
- `go/pkg/acsassert/go_tests_env_integration_test.go` — proves the child sees no EVOLVE_ key of the caller.
- `go/internal/acsrunner/runner.go` — go test spawn moved onto the projection.
- `go/internal/acsrunner/fail_closed_test.go` — follows the runner env change.
- `go/internal/acsrunner/lanebudget_test.go` — follows the runner env change.
- `go/internal/acsrunner/runner_test.go` — follows the runner env change.
- `go/internal/core/task_contract.go` — go test spawn moved onto the projection.
- `go/internal/phases/ship/repocontract.go` — go test spawn moved onto the projection.
- `go/internal/phases/ship/repocontract_pack_test.go` — follows the repo-contract env change.
- `go/internal/releasepreflight/releasepreflight.go` — go test spawn moved onto the projection.
- `go/internal/repocontract/repocontract.go` — adds internal/ipcenv to Packages().
- `go/internal/sysexec/sysexec.go` — env support for the projected spawn.
- `go/internal/sysexec/sysexec_cover_test.go` — covers the sysexec change.
- `go/acs/cycle1997/predicates_test.go` — TDD-phase predicates for acceptance criteria 1 to 3.
- `.evolve/evals/go-test-env-projection-pinned-lane.md` — one score cap for the decoy projection shape.
- `docs/private/research/archived-2026-10-20/superseded-predicate-packages/cycle1978/predicates_test.go` — archived cycle 1978 predicates, which named tests that do not exist.
- `docs/private/research/archived-2026-10-20/unshipped-build-explanations/cycle-1978-01m59yay55w9phzzhd318zkyb6.md` — archived explanation of the unshipped cycle 1978.

## Design Decisions
One syntactic rule inside the existing AST scan, instead of data-flow analysis: a projected spawner holds a projection call and no ambient os.Environ() call outside one. Rejected: tracking each cmd.Env assignment, because the scan does not resolve receivers.

## Verification
The ipcenv and pkg/acsassert packages pass with -count=1. The cycle 1997 predicates pass, including the 7 planted shapes. The real tree still passes the pin.

## Compatibility
Production code changed in 8 files. ipcenv.GoTestEnv is a new exported function. sysexec.WithEnv is a new exported wrapper that fixes the env of a RunFunc. The unexported acsrunner runCommander type takes an env argument, so Run passes lanebudget.Env(ipcenv.GoTestEnv()) to its spawn. core, ship and releasepreflight pass the same projection and keep their behavior: the env is still scrubbed of EVOLVE_ keys. The one behavior change is in pkg/acsassert.GoTests: its child go test no longer inherits the EVOLVE_ keys of the caller. internal/ipcenv joins repocontract.Packages(), so the pack runs its tests.

## Limitations
A spawner that reads os.Environ() for a purpose other than its env would need an exemption entry.
