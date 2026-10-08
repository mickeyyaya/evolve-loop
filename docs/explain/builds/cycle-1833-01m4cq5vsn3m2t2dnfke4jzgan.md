# Build Explanation — Cycle 1833

## Build Binding
- Cycle: 1833
- Base SHA: 8d6f2bbabb90a471842fbe1d2fc1808feaacba84

## Summary
Release preflight now fails the naming step when it cannot stat `.evolve/naming.json` for any reason other than not-exist. A permission error or a symlink loop no longer passes as an absent manifest. Named unit tests now pin the two fail-open rules of the package: a missing manifest passes, and a CI lookup failure is the unavailable verdict. The two why comments that held these rules are gone. The simulation-suite failure warning no longer names the stale v12.1.5/v12.2.0 roadmap.

## Rationale
`defaultNameGuard` read every `os.Stat` error as "no manifest". An unreadable manifest therefore skipped the naming guard without a sound. Only `os.ErrNotExist` means that there is nothing to guard. Every other error must fail the step, so the release cannot pass a guard that never ran. Tests now state the fail-open rules, so the comments became redundant. Code carries no comments (AGENTS.md invariant 10).

## Changed Areas
- `go/internal/releasepreflight/releasepreflight.go` — `defaultNameGuard` returns nil only for `os.ErrNotExist`. It wraps every other stat error as `naming manifest: %w`, which keeps `fs.ErrPermission` and the path. The why comments above `defaultNameGuard` and `defaultCIConclusion` are deleted.
- `go/internal/releasepreflight/preflight_run.go` — the simulation failure warning reads `WARN: auto-respond simulation suite failed (advisory): <err>`.
- `go/internal/releasepreflight/releasepreflight_test.go` — the TDD phase changed one expected log string in `TestRun_SimulationAdvisory` to match the new warning.
- `go/internal/releasepreflight/fail_open_pins_test.go` — new unit tests from the TDD phase. They pin the missing-manifest pass, the stat-error failure (EACCES, ELOOP) and the CI unavailable verdict.
- `go/acs/cycle1833/predicates_test.go` — the cycle predicates from the TDD phase. They drive `releasepreflight.Run` through the default seams.
- `.evolve/evals/releasepreflight-fail-open-pins.md` — the eval from the TDD phase. It lists the score caps of the four graders.
- `docs/architecture/packages/internal-releasepreflight.md` — the package page names the new boundary and the pinning tests.

## Design Decisions
The error is wrapped with `%w` and not returned raw. The step error then says which input failed, and callers can still match `fs.ErrPermission`. The naming step already turns a guard error into `ErrCheckFailed`, so the failure needs no new code path. The CI lookup keeps its fail-open behavior unchanged. Absent `git`/`gh` tooling must not block a release, and the new test makes that behavior explicit.

## Verification
`go test -count=1 ./internal/releasepreflight` passes. All six `go/acs/cycle1833` predicates pass. `evolve acs suite --cycle 1833` reports green=177 red=0. `gofmt -l` and `go vet ./...` are clean.

## Compatibility
No exported symbol, flag or option changed. A repository without a naming manifest behaves as before. A repository whose manifest cannot be stat'ed now fails preflight step 5 where it passed before. That is the intended correction.

## Limitations
The EACCES case is skipped when the tests run as root. The ELOOP case still covers the boundary there. The stale prose in `docs/history/code-comments/internal-releasepreflight.md` is a historical record and stays unchanged.
