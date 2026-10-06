# `internal/repocontract`

Ship's repo-contract fixed scanner pack, in one place: which suites it runs and whether it runs for a tree. Ship's gate (`internal/phases/ship/repocontract.go`) and the build handoff floor (`internal/core/repo_contract_floor.go`) both read it.

## What it owns

- **`Packages()`** is the pack's suite list and its one home. Ship projects it into `repoContractPackages`, and the red message names each suite from it. The list holds the catalog, profile and phase-coherence scanners, the routing tests, and the packages the detector below finds reading the whole tree that are cheap enough to run at every ship: the raw git fixture ratchet, the function-size ratchet, the `TestMain` exit check (`testmainexit`), the env-agnostic param check (`policy`), the trust-kernel call-site scans (`guards`), the ACS tag guard (`acssuite`), fleet's module-graph partition (`fleet`), the unsatisfiable-predicate lint's live-bytes test and its `go/acs` corpus sweep, which fails only on the two proof kinds (`inverted-idiom`, `go-run-exit-code`) and logs the `absence-message` heuristic, so a heuristic false positive in a lane's new `go/acs/cycle<N>` package never blocks its ship (`evalqualitycheck`, under half a second) and this package.
- **Test-only helpers stay out of production code.** `TestTestOnlyHelpersAreNeverImportedByProductionCode` walks the module's non-test Go files (skipping `testdata`, `vendor` and dot directories) and fails on any import of `internal/fakeclitest` or `internal/tmuxtest`. `fakeclitest`'s `init` runs whatever script sits beside the running binary, and `tmuxtest.Main` starts and kills tmux servers, so either in a production binary would be a trust-boundary hole. It runs at every ship because this package is in the pack. `TestProductionImporters_FlagsOnlyNonTestFiles` pins that a `_test.go` file or a `testdata` fixture importing a helper is not flagged.
- **`PackRuns(gate, root)`** decides whether the pack runs: only while `gates.repo_contract_gate` is on and `<root>/go/go.mod` declares evolve-loop's own module. **`GateOn`** reads the dial, and **`ModuleDir`** gives the module directory.

## Why the list is small and fixed

Every suite costs wall time at every ship, every build handoff and every `evolve selfcheck build`, and it must carry the same near-zero false-red rate, because a red fails the lane. The pack runs as one `go test` invocation over its packages in parallel, so the cost of an addition is bounded by the slowest member, not added on top.

## Why a test that reads the whole tree must run before main

Changed-scope testing selects the packages a lane changed and the packages that import them. A test that reads the whole tree breaks when any file changes. A lane that changes neither its package nor an importer can therefore still break it, and only main's CI would find out.

Cycle 1779 did exactly that (2026-09-30). Its lane ship grew `cmd/evolve.runCycleRun` past the function-size ratchet. The pack held the raw-git ratchet but not the size ratchet, so main went red for every open PR.

## How the list is kept honest

`TestPackages_HoldEveryTestThatReadsTheWholeTree` reads every tracked test file through `rawgitratchet.BoundTestFiles`. A new test file is found once it is tracked or staged. It finds each package whose tests leave their own directory for the tree, in either of two ways:

- **A climb out of the package.** A call whose string-literal path parts, joined and cleaned as `filepath.Join` does (so `./..`, empty parts and a name followed by `..` resolve as they would at run time), lead out of the package's directory, in one of two shapes:
  - It lands on the module root, with or without a subpath after it: `filepath.Abs("../..")`, `filepath.Join(filepath.Dir(self), "..", "..", "acs")`, `PartitionGraph(todos, 2, "../..")`.
  - It lands on a top-level directory with nothing after it, as when `explanationCallSiteFiles("..", vocab)` walks all of `go/internal`.

  Inside a `Join`, a leading `Dir(...)` call is taken to be the package's own directory, any other non-literal before the climb is an unknown base (so the call is not a climb from the package), and a non-literal after it counts as a subpath. Any other call ignores its non-literal arguments, as `PartitionGraph(todos, 2, "../..")` and `explanationCallSiteFiles("..", vocab)` show. `strings` and `bytes` calls are string matching, not paths, so they are ignored.
- **A walk up to `go.mod`.** A bare `for { … }` loop that names `"go.mod"`. A `range` or counted loop naming it writes fixtures; it does not climb.

Each package it finds must be in `Packages()` or recorded in the test's `readsTheTreeOutsideThePack` table with its reason, and `packProblems` fails on a record that no longer applies. The table holds the large packages whose seam or single-writer tests read the tree (`cmd/evolve`, `core`, `phases/ship`, `phases/runner` and others). Running each of them whole at every ship is too slow, so they wait for test-level selection (inbox `repo-contract-test-level-selection`). `TestClimbsOutOfItsPackage`, `TestWalksUpToGoMod` and `TestPackProblems` pin each shape, the negative ones included. The test itself is in the pack, so it runs before main does.

The detector knows these shapes and no others. A test that finds the tree another way is not caught until someone adds its shape here.

## What the detector skips

- **`acs/`.** Main's required CI runs only `-tags acs ./acs/regression/...` (`.github/workflows/ci.yml`), and the audit's CI-parity gate replays that suite for every lane (`internal/phases/audit/ciparitygate`), so a lane that breaks a regression predicate is stopped before ship. The per-cycle predicates under `acs/cycleN` (and `acs/redteam`) are `//go:build acs` and never run in required CI, so they cannot turn main red.
- **Tests that climb past the module root** into the repository's `agents/`, `docs/` or `skills/`. A lane's non-Go edit can break those, but that is a different class: a changed non-Go file mapped to the tests that read it.
