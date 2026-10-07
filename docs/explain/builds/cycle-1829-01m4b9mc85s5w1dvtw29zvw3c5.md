# Build Explanation — Cycle 1829

## Build Binding
- Cycle: 1829
- Base SHA: 63eb3af769020bbb87239d4fd42f4536c9603769

## Summary
The repo-contract pack now also runs, by test name, the 50 tests in 13 large packages that read the whole tree, so a lane that adds a second `verdict.New(` site is stopped at the build floor and does not first red main. Every `go test` the pack starts now runs in its own process group, and a cancel kills that whole group, so a cancelled pack no longer leaves its test binary running.

## Rationale
The 13 packages (`cmd/evolve`, `core`, `phases/runner` and others) were recorded outside the pack because running them whole at every ship is too slow. Their seam and single-writer tests read the whole module, so a lane that touched neither the package nor an importer could break one and only main's CI noticed. Running only the tree-reading tests by name gives the pack that coverage: the live pack finished green in about 30s cold, inside the floor's 120s deadline. `exec.CommandContext` killed only the `go` command. A cancel at the floor's deadline therefore orphaned the test binary until its own 20m `-test.timeout`. Killing the process group is the pattern `internal/cliupdate`'s `GroupRunner` already uses.

## Changed Areas
- `go/internal/repocontract/repocontract.go` — adds `TestSelection` and `TreeReadingTests()`, the pack's by-name half, holding the 50 tree-reading tests of the 13 packages. It returns a fresh copy on every call, as `Packages()` does.
- `go/internal/repocontract/packages_test.go` — the detector now works per declaration and credits each `Test…` function that reaches a climb or a `go.mod` walk through helpers or package-level vars. `packProblems` checks the selections against it exactly, and the `readsTheTreeOutsideThePack` escape table and its reason constant are removed.
- `go/internal/repocontract/packages_detector_test.go` — rewrites `TestPackProblems` for selections and adds `TestReadingTestsOf_CreditsTheTestsThatReachARead` (credit through a helper, with `TestMain` and non-tests left out) and `TestTreeReadingTests_SelectsOnlyPackagesOutsideThePack`.
- `go/internal/phases/ship/repocontract.go` — `defaultRepoContractTest` runs the whole packages, then the selections as one anchored `-run` over their packages, and merges the two outcomes (`packOutcome.merged`). `runGoTestJSON` sets the process-group cancel and a `WaitDelay`, and ship's scan log names the by-name command.
- `go/internal/phases/ship/repocontract_cmd_unix.go` — sets `Setpgid` and a `Cancel` that SIGKILLs `-pid`, so a cancel takes the test binaries with it.
- `go/internal/phases/ship/repocontract_cmd_other.go` — a no-op on platforms without process groups, which keep exec's default kill.
- `go/internal/phases/ship/repocontract_pack_test.go` — pins the selection argv (it uses the budgeted builder, names each test once, then lists the packages) and the merge (reds and output from both runs are kept, a cancel stays ambiguous).
- `go/acs/cycle1829/predicates_test.go` — the TDD phase's five predicates. They drive the production floor over the production pack, and this build turns them green.
- `.evolve/evals/repo-contract-cancel-orphans-test-binary.md` — the TDD phase's eval for the cancel half.
- `.evolve/evals/repo-contract-test-level-selection.md` — the TDD phase's eval for the by-name half.
- `docs/architecture/packages/internal-repocontract.md` — documents `TreeReadingTests()`, the per-declaration detector and the empty outside set.
- `docs/architecture/packages/internal-phases-ship.md` — documents the by-name run and the process-group cancel.

## Design Decisions
- The selection is a declared list, not computed when the pack runs. Moving the AST detector into production code would make the pack select itself, but that is a larger change. A declared list keeps the pack reviewable, and the detector test, which is in the pack, fails on any drift in either direction.
- One `go test` with a single anchored alternation covers all 13 packages. Thirteen per-package invocations would each link their own binary in sequence. A name shared by two selected packages runs in both, which adds tests and never drops one.
- The outside table is deleted rather than left empty. Any package can now be selected by name, so the escape hatch no longer has a reason to exist. A package whose read no test reaches must go into `Packages()` whole.
- `WaitDelay` is set on every pack run on every platform. The process-group split covers only `Setpgid` and `Cancel`, following the `cliupdate` grouprunner files.

## Verification
- `go test -tags acs -count=1 ./acs/cycle1829`: 5/5 PASS (001 no surviving test binary, 002 a cancel names nothing, 003 the second `verdict.New(` reds the floor, 004 the table is empty and each of the 13 packages runs a test, 005 the slow packages run by name and the live pack is green within the deadline).
- `go test -count=1 ./internal/repocontract ./internal/phases/ship ./internal/sizeratchet` passes, along with the full module run recorded in the build report.

## Compatibility
`Packages()` and `repoContractPackages` are unchanged, so the importer backstop's exclusions and the red message's suite list stay byte-identical. The pack's verdict classes are also unchanged: a named red, green, or an ambiguous exit that is retried and then classed infra.

## Limitations
A tree-reading shape the detector does not know (see the package page) is still not selected. Windows does not build this module today (`internal/adapters/flock`), so the `!unix` file is checked by review only.
