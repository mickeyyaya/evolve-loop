# ADR-0110 — Every test that reads the whole tree runs before main, and a detector keeps that list complete

- **Status:** Accepted (2026-09-30, PR #751, `2fe8aa908`)
- **Supersedes nothing.** Extends the ship's repo-contract scanner pack and the build handoff floor that runs it.
- **Related:** [the incident that motivated it](../../incidents/2026-09-30-a-lane-ship-grew-a-function-past-the-size-ratchet.md); [`internal/repocontract` design notes](../packages/internal-repocontract.md); [runtime reference, "Ship repo-contract gate"](../../operations/runtime-reference.md).

## Context

A lane's tests are chosen by change scope: the packages it changed, and the packages that import them. That selection is sound for ordinary unit tests. It misses one class entirely: a test that reads files its own package does not import, such as a ratchet that scans every function in the module, a guard that scans every call site, or a check over every test file. Such a test breaks when a file anywhere changes, yet a lane that changes neither its package nor an importer never runs it. Only main's CI does.

The ship already had a fixed scanner pack for this class, but its list was kept by hand. On 2026-09-30, cycle 1779's lane ship grew `cmd/evolve.runCycleRun` past the function-size ratchet. The ratchet was not in the pack, and main went red for every open PR. A search for the rest of the class found five more whole-tree tests outside the pack (`testmainexit`, `policy`, `guards`, `acssuite`, `fleet`) and thirteen large packages with seam tests of the same kind.

## Decision

1. **One list.** The pack's suite list lives in `repocontract.Packages()`. Ship projects it into `repoContractPackages`, the red message names each suite from it, and the docs point to it instead of copying it.
2. **Whole-tree tests that are cheap to run join the pack.** The pack runs its packages in one parallel `go test`, so its wall time is set by its slowest member.

   | Package | What its test reads |
   |---|---|
   | `sizeratchet` | every function in the module, against its line allowance |
   | `rawgitratchet` | every tracked test file, for raw `git init` fixtures |
   | `testmainexit` | every test file, for a deferred cleanup before `os.Exit` in `TestMain` |
   | `policy` | the source of the param packages, for environment reads |
   | `guards` | every call site under `go/internal` for the build-explanation contract |
   | `acssuite` | every ACS predicate under `go/acs`, for its build tag |
   | `fleet` | the module's package graph |
   | `repocontract` | every tracked test file, for tests that read the whole tree (the detector below) |
3. **Completeness is detected, not remembered.** `TestPackages_HoldEveryTestThatReadsTheWholeTree` reads the syntax tree of every tracked test outside `acs/` and finds each package whose tests leave their directory for the tree, in one of two ways:
   - a call whose literal path parts, joined and cleaned as `filepath.Join` does, land on the module root (with or without a subpath), or on a top-level directory with nothing after it;
   - a walk up to `go.mod` in a bare `for` loop.

   Each package found must be in the pack or recorded in `readsTheTreeOutsideThePack` with its reason. A record that no longer applies also fails the test.
4. **The detector lives in the pack.** A detector anywhere else would be one more whole-tree test that no lane selects.
5. **Large packages wait for test-level selection.** Thirteen packages whose seam or single-writer tests read the tree are recorded outside the pack, since running each of them whole at every ship is too slow. Inbox `repo-contract-test-level-selection` tracks moving them in as test-level entries.

## Alternatives considered

| Alternative | Why it was not chosen |
|---|---|
| Add the size ratchet to the hand-kept list and move on | Fixes the instance. The next whole-tree test would repeat the incident. |
| Require every `internal/*ratchet` package to be in the pack | The first reviewed version. It keys on a name, and `testmainexit` has the same property under a different name. |
| Detect one spelling of the climb (`filepath.Abs("../..")`) | The second reviewed version. It missed a walk up from `runtime.Caller`, a one-step climb onto `go/internal` and a climb with a subpath. |
| Put the detector in `phases/ship`, next to the pack's runner | The detector would itself be a whole-tree test outside the pack. |
| Run every package's tests at every ship | Too slow for every ship and every build handoff. |

## Consequences

- A lane that breaks a whole-tree test is stopped at its build handoff or its ship, not on main.
- A new whole-tree test fails `repocontract`'s own test until it joins the pack or is recorded. The failure message names the package and both remedies.
- The detector knows two shapes. A test that finds the tree another way goes undetected until its shape is added. The package doc states this limit.
- The recorded packages remain a known gap until test-level selection lands. Their seam tests still run only when their own package or an importer changes.
- Tests that climb past the module root into `agents/`, `docs/` or `skills/` form a separate class: a changed non-Go file mapped to the tests that read it. This ADR does not cover it.

## Evidence

- Four architecture-review rounds, each recorded in PR #751. Each round widened the fix from the instance towards the class.
- Mutation sweeps: 6 mutants on the moved flag and root lines and 8 on the detector's rules, all killed.
- The full floor (`make test` over 247 packages, integration, e2e, durable ACS, apicover, cover-strict) passed on the final tree, and main's required CI passed on `2fe8aa908`.
