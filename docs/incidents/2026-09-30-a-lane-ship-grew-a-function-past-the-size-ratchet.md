# A lane ship grew a function past the size ratchet, and main went red (cycle 1779, 2026-09-30)

## What happened

Cycle 1779 (wave 53) shipped a three-item batch as `461aa782a` at 18:46 local time. Its build grew two functions in `go/cmd/evolve`:

| Function | Size after the ship | Limit |
|---|---|---|
| `runCycleRun` | 91 lines | allowance 86 in `go/internal/sizeratchet/offenders.json` |
| `runCycleHealth` | 51 lines | 50, the ratchet's default limit |

Every gate the lane ran passed, and the ship landed. Main's required CI then failed on `internal/sizeratchet` (`TestRatchet_ModuleFunctionsFitTheirAllowances`). Every PR opened or updated after that point inherited the red: #748's merge commit, and the round-12 comment PR (#749), whose Ubuntu jobs were the first to show it in review.

Main stayed red for about an hour and a half, until #751 merged at 20:20 and main's required CI passed on `2fe8aa908`. No loop wave ran in that window, so no lane shipped onto the red main.

## Why the lane's gates did not catch it

Before a lane ships, its tests are chosen by change scope: the packages the lane changed, and the packages that import them. `internal/sizeratchet` is neither for a change to `go/cmd/evolve`. Its test reads every function in the module, but nothing in the module imports it, so no lane that leaves it untouched ever selects it.

The ship's fixed scanner pack exists for exactly this kind of test. A test that reads the whole tree breaks when any file changes, so it has to run before main does. The pack held the raw-git fixture ratchet (`rawgitratchet`) but not the size ratchet. The pack's list was kept by hand, so a whole-tree test joined it only when someone remembered to add it.

Looking for the rest of the class turned up more whole-tree tests that no pre-main gate ran:

| Package | What its test reads |
|---|---|
| `internal/sizeratchet` | every function in the module |
| `internal/testmainexit` | every test file, for `defer` before `os.Exit` in `TestMain` |
| `internal/policy` | the source of the param packages, for environment reads |
| `internal/guards` | every call site under `go/internal` |
| `internal/acssuite` | every ACS predicate under `go/acs`, for its build tag |
| `internal/fleet` | the module's package graph |

Thirteen larger packages (`cmd/evolve`, `core`, `phases/ship` and others) hold seam or single-writer tests that also read the tree. They are too slow to run whole at every ship.

## The fix (#751)

- **Main green again.** `runCycleRun` parses its flags in `parseCycleRunFlags` into a `cycleRunFlags` value and builds its request with `cycleRunFlags.request`, which brings it to 62 lines; its allowance tightened from 86 to 62. `runCycleHealth` resolves its root in the new `cycleHealthRoot` and is now 44 lines. New tests pin the flag parsing, the request, the `--simulate` wiring and the root resolution.
- **The class, by detection rather than memory.** The pack's list moved to `repocontract.Packages()`, and ship projects it. It gained the six packages above and `repocontract` itself. `TestPackages_HoldEveryTestThatReadsTheWholeTree` reads the syntax tree of every tracked test and finds each package whose tests climb out of their own directory onto the module root, or walk up to `go.mod`. Each one must be in the pack or recorded outside it with a reason. The thirteen large packages are recorded, waiting for test-level selection (inbox `repo-contract-test-level-selection`). The detector sits inside the pack, so it runs before main does.

The decision and its alternatives are in [ADR-0110](../architecture/adr/0110-whole-tree-tests-run-before-main.md). The rule, the shapes the detector knows and its stated limits are in [the package doc](../architecture/packages/internal-repocontract.md).

## How the fix was reviewed

The architecture review took four rounds, and each one moved the fix closer to the class:

1. The first pin keyed on the package name `*ratchet`. It missed `testmainexit`, which has the same property and a different name.
2. The next detector keyed on how a test spelled its climb (`filepath.Abs("../..")`). It missed `policy` (a walk up from `runtime.Caller`), `guards` (a one-step climb onto `go/internal`) and `acssuite` (a climb with a subpath). Its own failure paths were also unpinned.
3. Three detector boundaries had no test.
4. The climb was rewritten to join and clean its path parts the way `filepath.Join` does, which deleted the one branch that could not be pinned. Every mutant the reviews raised is now killed.

## What we learned

- **Pin a class by its force, not by a name or a spelling.** The class here is "a test that reads files its package does not import". A name suffix or one call shape describes some members of the class, never all of them.
- **A guard must itself run before main.** A detector placed in `phases/ship` would have been another whole-tree test that no lane selects, so it lives inside the pack it checks.
- **Main red blocks everything downstream.** Two merged PRs and every open one inherited the failure within the hour. The ratchet fix went first, ahead of the rest of the boundary's work.

## References

- PR #751: `fix(ship): the fixed scanner pack runs the whole-tree tests a lane can break without touching them` (`2fe8aa908`)
- [ADR-0110](../architecture/adr/0110-whole-tree-tests-run-before-main.md)
- [`internal/repocontract` design notes](../architecture/packages/internal-repocontract.md)
- A related earlier case: cycle 1745's raw `git init` fixture passed the build and two audits and was first caught by ship's pack, which is why the build handoff floor now runs the pack too ([`internal-core` design notes](../architecture/packages/internal-core.md), Repo-contract floor).
