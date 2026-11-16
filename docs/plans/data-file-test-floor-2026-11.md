# Plan: a lane that changes a data file runs the tests that read that file

- **Status:** proposed, 2026-11-16. Revision 3, after design review round 2.
- **Inbox item:** `lane-floor-selects-the-tests-that-read-a-changed-data-file`.
- **Package notes:** [internal-repocontract](../architecture/packages/internal-repocontract.md), [internal-changedpkgs](../architecture/packages/internal-changedpkgs.md) and [internal-ciparity](../architecture/packages/internal-ciparity.md).
- **Decision:** this plan holds its decisions in §5. No ADR is necessary, because the plan extends one registry that already has an owner.
- **Research:** the facts in §2. They come from code reads at `origin/main` `b1a9d01af`, from the open pull request #1502 (`perf/two-stage-build-floor`, commit `70dbcfdf0`) and from `grep` counts over `go/` on 2026-11-16.

**Terms used throughout:**

- **Build floor:** all the floor judges that run at the build handoff. Two of them run tests: the unit-test floor and the repo-contract floor.
- **Unit-test floor:** `changedPackageFloorChecks`. It runs the changed Go packages whole.
- **Repo-contract floor:** `RepoContractFloorChecks` and `RepoContractFloorJudge`. They run the repo-contract pack, the same pack that ship runs.
- **Repo-contract pack:** `Packages()` run whole, then `TreeReadingTests()` run by name, in `defaultRepoContractTest`.
- **Data file:** a tracked file that is not a `.go` file, for example `.github/workflows/required.yml` or `go/Makefile`.
- **Data reader:** a test that opens a data file outside its own package directory.
- **Read key:** the repository-relative path, or the directory prefix, that a data reader opens.
- **`WholeRepository`:** the named read key `"/"` in `go/internal/repocontract/datafiles.go`. It means "this test reads the repository root as a tree", and it matches every path.

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)
11. [Review round 1 answers](#11-review-round-1-answers)

## 1. Request

The inbox item (2026-11-16) says:

> A lane that changes only a non-Go data file (example: `.github/workflows/required.yml`) does not run the test packages that read that file (example: `go/internal/ciparity`).

The cause, in the words of the item: "the lane floor selects test packages from changed Go packages only".

The two acceptance criteria of the item, quoted in three parts (parts 1 and 2 are criterion 1):

1. "a lane that changes only `.github/workflows/required.yml` runs `go/internal/ciparity` in its floor".
2. The proof is "a test through the real floor entry point that is red without the change".
3. "a guard fails when a test package reads a repo data file that no registry row names".

Its fix idea: one registry maps a path glob to the test packages that read it. The registry comes from the `os.ReadFile` and embed sites of each test package where possible. A guard stops a new reader that has no row. The lane floor and the ship gate run the packages that the registry selects.

### Worked example: a lane edits `required.yml` today

1. The lane changes one path: `.github/workflows/required.yml`.
2. The unit-test floor drops the path, because it does not end in `.go` (`go/internal/core/phase_bindings_selfcheck.go:66`). It runs no unit test.
3. The repo-contract floor runs the pack, because `PackRuns` does not look at the change (`go/internal/repocontract/repocontract.go:250-261`). The pack runs one `ciparity` test by name, `TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier` (`go/internal/repocontract/repocontract.go:157-159`).
4. At ship, the same pack runs again, with the same one `ciparity` test (`go/internal/phases/ship/repocontract.go:456-474`). The importer backstop prints "no Go change in the tree — skipped" (`go/internal/phases/ship/repocontract_importers.go:33-36`).
5. The `ciparity` tests that parse `required.yml`, for example `TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies` (`go/internal/ciparity/required_workflow_test.go:9-11`), first run in CI on main.

After this plan, step 3 and step 4 run every `ciparity` test that reads `.github/workflows/`, by name, in the pack.

## 2. Facts

### 2.1 The unit-test floor and the ship backstop read `.go` files only

**F2.1** `changedGoTestPackages` keeps a path only if it starts with `go/` and ends with `.go` (`go/internal/core/phase_bindings_selfcheck.go:62-78`). `floorTestPackages` is a wrapper on it (`go/internal/core/build_floor_retake.go:63-65`).
*Implication:* the unit-test floor cannot select a package from a data file.

**F2.2** `floorTestPackages` returns package names only, and the cover-changed floor also calls it (`go/internal/core/cover_changed_floor.go:161`). Its runner `buildSelfCheckRunner` takes a package and no test names (`go/internal/core/phase_bindings_selfcheck.go:26`, `:106-116`).
*Implication:* a data-file selection that goes through `floorTestPackages` joins the coverage check and loses its test names.

**F2.3** `changedpkgs.FileToPackage` requires the `.go` suffix (`go/internal/changedpkgs/changedpkgs.go:13-26`), and `PackagesOf` uses it (`go/internal/changedpkgs/changedfiles.go:47-63`). The ship importer backstop gets its packages from `PackagesOf`, skips when the list is empty, and runs whole packages (`go/internal/phases/ship/repocontract_importers.go:32-75`).
*Implication:* the importer backstop is the wrong seam for by-name tests of a data-only lane.

**F2.4** `ciparity.TierScope` removes `acs` patterns and env-exclusive packages from a floor list (`go/internal/ciparity/tierscope.go:17-33`). The only env-exclusive package is `internal/bridge` (`go/internal/ciparity/envexclusive.go:13-17`).
*Implication:* a data row in `internal/bridge` runs in the pack under a live wave, unless the plan filters it (D9).

### 2.2 The repo-contract pack already runs by-name tests at the floor and at ship

**F2.5** The build floor runs the pack for every lane: `RepoContractFloorChecks(repoContractPack)` in `productionBuildFloorChecks` (`go/cmd/evolve/cmd_cycle_config.go:45-50`), and the `repo-contract-floor` judge in the gate registry (`go/internal/gateregistry/seed_floors.go:20-24`). The judge has its own base attribution (`go/internal/core/repo_contract_floor.go:25-45`, `:54`).
*Implication:* a by-name test in the pack already reaches the floor through the real floor entry, with one retake and attribution path.

**F2.6** The pack runs `Packages()` whole, then the by-name selections in one `-run '^(…)$'` call (`go/internal/phases/ship/repocontract.go:81-84`, `:97-106`). Ship runs it in `runFixedPack` before the two backstops, and `PackRuns` gates it on policy and the module path only (`go/internal/phases/ship/repocontract.go:440-474`).
*Implication:* the pack is the one seam that runs by-name tests at the floor and at ship, also when no Go file changed.

**F2.7** Pull request #1502 (S6) splits the build floor. `productionFloorCheck` gets `Run: productionStaticFloorChecks` and `Tests: productionSuiteFloorChecks`. `productionSuiteFloorChecks` runs `SuiteBuildFloorChecks` and then `RepoContractFloorChecks(repoContractPack)` (`go/cmd/evolve/cmd_cycle_config.go:53-59` on `perf/two-stage-build-floor`). `BuildHandoffFloor.Failures` runs the `Tests` field only after the static checks pass (`go/internal/core/build_handoff_floor.go:39-50` on that branch).
*Implication:* after S6, the pack runs in stage B as part of the `Tests` entry. A change inside the pack needs no new wiring for S6.

**F2.8** The pack has a 120 s deadline at the floor (`go/internal/core/repo_contract_floor.go:17`). The pack takes "about 30s cold" today (`docs/architecture/packages/internal-repocontract.md:14`).
*Implication:* the data rows have about 90 s of room in the floor deadline. G4 measures the real cost.

### 2.3 The tree-reading registry already exists, and it skips this class on purpose

**F2.9** `TreeReadingTests()` returns `TestSelection{Package, Tests}` rows (`go/internal/repocontract/repocontract.go:41-52`), from `treeReadingSelections` (`go/internal/repocontract/repocontract.go:54-240`). `Packages()` lists 15 whole packages (`go/internal/repocontract/repocontract.go:21-39`).
*Implication:* the repository has one home for "tests that a lane can break without a change to their package". This plan extends that home.

**F2.10** The guard `TestPackages_HoldEveryTestThatReadsTheWholeTree` fails on a whole-tree reader with no row, and on a row that no longer reads (`go/internal/repocontract/packages_test.go:16-61`). It also pins a list of packages that the detector must find, so a blind detector fails (`go/internal/repocontract/packages_test.go:19-23`).
*Implication:* the new guard copies the two-way check and the "must find" pin.

**F2.11** The detector counts a climb only if it lands on the module root (`go/internal/repocontract/packages_test.go:186-197`). The case "past the module root" is a pinned negative (`go/internal/repocontract/packages_detector_test.go:24`). The package note says that such tests are "a different class: a changed non-Go file mapped to the tests that read it" (`docs/architecture/packages/internal-repocontract.md:40`).
*Implication:* the owner of the registry already named this gap and its shape.

**F2.12** `TestNoTestWritesIntoTheRealTree` follows a path root through helpers: a `..` climb, `os.Getwd()`, the file of `runtime.Caller` (`go/internal/repocontract/realtree_writes_test.go:353`) and `git rev-parse --show-toplevel` (`go/internal/repocontract/realtree_writes_test.go:459`). It fails if it finds no root helper, so a blind scan is red (`go/internal/repocontract/realtree_writes_test.go:13-17`). It does not resolve a method call on a root (`docs/architecture/packages/internal-repocontract.md:54`).
*Implication:* the read detector reuses this root analysis. A root that flows into a call that the analysis cannot follow needs a fail-closed rule (D6).

**F2.13** `ciparity` finds the repository root through a helper, `workflowRepoRoot` (`go/internal/ciparity/workflows_test.go:190-197`). It joins `.github/workflows` and a file name, for example the constant `RequiredWorkflow` (`go/internal/ciparity/macos_weekly_test.go:182`, `go/internal/ciparity/ciparity.go:16`). `readWorkflowContract(t, name)` reads with a parameter tail (`go/internal/ciparity/workflows_test.go:177-179`).
*Implication:* the detector must follow helper roots. A parameter tail gives the directory prefix `.github/workflows/`.

### 2.4 Measured counts (2026-11-16, `grep` over `go/`)

| Measure | Count | Method |
|---|---|---|
| Test packages | 1231 | unique directories with a `_test.go` file |
| Test packages with `os.ReadFile` in a test file | 560 | unique directories from `grep -rl 'os.ReadFile' --include='*_test.go'`; most read `t.TempDir()` fixtures |
| Directories with a `//go:embed` line | 10 | 8 production, 1 test file, 1 `testdata` fixture; each embeds files inside its own directory |
| Test packages with a `..` text literal | 122 | unique directories of the 328 test files that `git grep -l -E '"\.\./\|"\.\."' b1a9d01af` lists |
| Test packages that climb past the module root | 51 | `..` tokens in one line greater than the package depth |
| Test packages that run `git rev-parse --show-toplevel` | 20 | `grep -rl 'show-toplevel'` |
| Candidate repository-root readers (union of the two rows above) | 66 | 13 under `go/acs`, 53 outside it |
| Candidates that the tree-reading registry names | 15 | intersection with the 36 registry packages (15 in `Packages()`, 21 selection rows) |
| Candidates that the registry does not name | 51 | 38 outside `go/acs` |
| Registry rows keyed by a data path | 0 | `TestSelection` has no path field |
| `ciparity` tests that the pack runs | 1 of 138 | `go/internal/repocontract/repocontract.go:157-159`; `grep -c '^func Test'` |

*Implication:* the counts are an upper bound, because a `show-toplevel` call can serve a temporary repository. The detector in C2 gives the exact set.

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| G1. A data-only lane runs the data readers in its build floor. | `TestRunRepoContractPack_RunsEveryDataReadRowByName` | red before C3, green after |
| G2. The ship gate runs the same data readers, also when no Go file changed. | `TestRunRepoContractGate_RunsEveryDataReadRowByNameWithNoGoChange` | red before C3, green after |
| G3. A new data reader cannot skip its row. | `TestDataFileReads_EveryRepoDataReadHasARow` | fails on an unnamed read, a stale row and a blind detector |
| G4. The pack stays inside its floor deadline. | pack wall time at the floor, cold, and the count of by-name tests | 90 s or less, against the 120 s deadline (F2.8); the count under `byNameTestCap` |

Non-goals:

- The plan does not change CI routing in `.github/workflows/required.yml`. CI stays the last backstop.
- The plan does not select `go/acs` predicates. The audit ACS run owns them.
- The plan does not change the unit-test floor, `floorTestPackages` or the cover-changed floor (F2.2).
- A changed data file inside the package directory of its reader (a `testdata` or embed file) is out of scope. Its fix touches `floorTestPackages`, which the cover-changed floor shares. It gets its own inbox item.
- The plan does not replace `TreeReadingTests()`. Go-tree readers keep their rows.

## 4. Candidate designs

- **A. Unit-test floor path.** Add the rows to `floorTestPackages`, and add them to the ship importer backstop targets. This was revision 1 of this plan.
- **B. Derive the readers at run time.** The floor parses every test package for each lane and selects the readers of the changed path.
- **C. Coarse rule.** Any changed data file outside `go/` selects every repository-root reader package whole.
- **D. A new registry in `go/internal/changedpkgs`.** The same rows as E, in the package that selects Go packages.
- **E. The pack runs the data rows by name.** The rows live in `go/internal/repocontract`. The pack adds every data row to its by-name `-run` call, beside the tree-reading rows.

Each criterion has a score from 1 (bad) to 5 (good).

| Design | Catches `required.yml` → `ciparity` | Cost for a lane | Guard strength | Build cost | One runner and one home | Score |
|---|---|---|---|---|---|---|
| A. Unit-test floor path | 4 | 4 | 5 | 2 | 1 | 16 |
| B. Derive at run time | 4 | 3 | 4 | 2 | 3 | 16 |
| C. Coarse rule | 5 | 1 | 2 | 4 | 2 | 14 |
| D. New registry in `changedpkgs` | 4 | 4 | 4 | 2 | 1 | 15 |
| E. Pack runs the data rows | 5 | 3 | 5 | 5 | 5 | 23 |

E wins on build cost and on one runner. The pack already runs at the floor and at ship, also with no Go change (F2.5, F2.6). It already runs by name, with one `-run` builder and one attribution path. A loses because its runner has no test names and pulls the rows into coverage (F2.2), and its ship seam exits early (F2.3). E costs more for each lane than a path selection, because every row runs every time. G4 bounds that cost.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Put the rows in `go/internal/repocontract/datafiles.go`, beside `TreeReadingTests()`. | F2.9, F2.11 | A new package: it is a second registry for one concern (design D). |
| D2 | A row is `DataFileRead{Path string; TestSelection}`. `Path` is one file, a directory prefix that ends in `/`, or `WholeRepository`. | F2.13: the detector derives a file or a prefix. | `path.Match` globs: `*` does not cross `/`, and the detector never produces a glob. |
| D3 | A row names tests, as `treeReadingSelections` does. | F2.9: `cmd/evolve` and `core` run by name for cost. | Whole packages: a lane then runs all of `cmd/evolve`. |
| D4 | `repocontract.ByNameTests()` merges the tree-reading rows and the data rows per package. The pack uses it in `defaultRepoContractTest`. | F2.5, F2.6 | A second by-name runner at the floor or at ship: two `-run` builders and two retake rules (design A). |
| D5 | The pack runs every data row on each run. It does not select rows by the changed paths. | F2.6: `RepoContractPackFn` has no paths; F2.8: the deadline has room. | Path selection now: it changes the signature of the pack at the floor and at ship before a cost exists. |
| D6 | The detector is fail-closed. A root that flows into a call that it cannot resolve counts as a read, with the prefix known at that call. | F2.12: methods and calls into other packages are not followed. | A list of known shapes only: a new shape skips its row with no red. |
| D7 | A read with no known suffix after a repository root gets a data row keyed `WholeRepository`. The guard refuses only the literal `""` as a `Path`. | The module-root detector does not see a root past the module root (F2.11), so `TreeReadingTests()` calls such a row stale (`go/internal/repocontract/packages_test.go:53-58`). | A `TreeReadingTests()` row: the two guards then disagree, and no row passes both. A union of the two detectors: it changes a guard that works today. |
| D8 | A read of `.go` files, or of the module root as a tree, stays in the tree-reading class. A read inside the own package directory needs no row. | F2.10, F2.11 | One merged class: it changes a guard that works today. |
| D9 | The ship consumer leaves out a data row of an env-exclusive package, where it builds `repoContractSelections`. The guard still requires the row. | F2.4; `go/internal/repocontract` imports no internal production package today, so the registry stays a leaf. | A filter in `ByNameTests()`: it adds an edge from the registry to the tier policy. A pack run of `internal/bridge`: its tmux boots time out under a live wave. |
| D10 | A data reader inside a pack package (`Packages()`) needs no row, because the pack runs that package whole. | F2.6 | A row there: the test runs twice. |
| D11 | A test caps the count of by-name tests at `byNameTestCap`. Its message names the revisit of D5. | The floor judge fails open: a pack that passes its deadline with no named red gives a WARN only (`go/internal/core/repo_contract_floor.go:40-41`, `go/internal/core/base_red_floor.go:32-38`). | A wall-time measure only: one measure does not stop the rows from growing later. |
| D12 | The WARN line of an unnamed pack exit names the pack wall time and the deadline. | The same fail-open path (D11). | No change: a floor that times out stays silent about the cause. |

We revisit D5 if the pack takes more than 90 s cold at the floor. We also revisit it if the rows need more than `byNameTestCap` tests (G4). Then the pack gets the changed paths and selects rows by `Path`, which D2 already allows. `WholeRepository` then matches every path, as D7 needs.

`byNameTestCap` starts at the count after the first C2 rows plus 20 %. Today the tree-reading rows name 143 tests.

Consequences:

- Good: a data-only lane gets the same red as main CI, at the floor and at ship, before it merges.
- Good: S6 needs no change. The pack runs in stage B of #1502 (F2.7).
- Cost: each lane runs every data row, also when it changes no data file. G4 bounds the cost.
- Cost: an author who adds a data reader also adds a row. The guard message names the row to add.

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| C1 | `DataFileRead` type, `WholeRepository` constant, `DataFileReads()` accessor and `ByNameTests()` merge in `go/internal/repocontract/datafiles.go`. | `TestByNameTests_MergesTheTreeAndDataRowsPerPackage` | console |
| C2 | Read detector, guard and cost cap in `go/internal/repocontract/datafiles_test.go`, with the first rows from the detector output. | `TestDataReadProblems`, `TestDataFileReads_EveryRepoDataReadHasARow` and `TestByNameTests_StaysUnderTheCap` | console |
| C3 | `repoContractSelections` in `go/internal/phases/ship/repocontract.go:32` takes `repocontract.ByNameTests()` without env-exclusive rows (D9). The by-name call in `defaultRepoContractTest` (`:83`) goes through the `goTestJSONFn` seam (`:166`), which today serves only the alone re-run. The log label at `:472` changes from "tree-reading tests by name" to "tree-reading and data-read tests by name". The WARN of `unnamedPackExit` names the wall time (D12). | `TestRunRepoContractPack_RunsEveryDataReadRowByName`, `TestRunRepoContractPack_ADocsOnlyLaneGetsTheSameByNameCall`, `TestRunRepoContractGate_RunsEveryDataReadRowByNameWithNoGoChange` and `TestRepoContractSelections_LeaveOutEnvExclusiveRows` | console |
| C4 | Package notes: replace the skip line at `docs/architecture/packages/internal-repocontract.md:40` with the data rows. Record D5, D6, D7, D9 (the filter is in the ship consumer) and `byNameTestCap`. | `evolve docs ste-lint --strict` and the link check | console |

`TestDataReadProblems` holds these cases:

- a read with no row fails;
- a row whose tests no longer read under its `Path` fails (stale row);
- a row in the wrong package fails;
- a parameter tail gives the directory prefix;
- a root passed to a method or to a function of another package counts as a read (D6);
- a root past the module root that flows into an unresolved call with no suffix needs a `WholeRepository` row (D7);
- the tree-reading guard and the data-read guard are both green on that `WholeRepository` row (D7);
- a data row with `Path` `""` fails (D7);
- a reader in a pack package passes with no row (D10).

The acceptance tests:

- **Acceptance 1:** `TestRunRepoContractPack_RunsEveryDataReadRowByName` in `go/internal/phases/ship`. `RunRepoContractPack` is the function that the real floor entry calls (F2.5, F2.7). The test makes a worktree that changes only `.github/workflows/required.yml` and records the `go test` arguments through the `goTestJSONFn` seam (`go/internal/phases/ship/repocontract.go:166`, wired by C3). It asserts `./internal/ciparity` and `TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies` in the by-name call. Before C3, the call names only `TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier`.
- **How the item's criterion is met:** the worktree changes only `required.yml`, the entry is the real floor entry, and the test is red before C3. The name says what the test pins, because under D5 the result does not depend on the change.
- **D5 pin:** `TestRunRepoContractPack_ADocsOnlyLaneGetsTheSameByNameCall` runs the pack on a worktree that changes only one file under `docs/`. It asserts the same by-name call as acceptance 1. A later change to path selection must change this test on purpose.
- **Ship side:** `TestRunRepoContractGate_RunsEveryDataReadRowByNameWithNoGoChange` drives `runRepoContractGateAt` on the `required.yml` worktree. It asserts the same by-name call, and the importer backstop line "no Go change in the tree — skipped".
- **Cost pin:** `TestByNameTests_StaysUnderTheCap` fails when `ByNameTests()` names more than `byNameTestCap` tests. Its message names the revisit of D5 (D11).
- **Acceptance 2:** `TestDataFileReads_EveryRepoDataReadHasARow` in `go/internal/repocontract`. It pins that the detector finds `internal/ciparity` with the key `.github/workflows/`, so a blind detector is red (F2.10, F2.12). It runs in the pack, so it runs at every ship.

## 7. Order

1. C1, because C2 and C3 call its types.
2. C2, because its detector output gives the first rows that C3 runs.
3. C3, because it closes acceptance 1 and the ship side with one line of wiring.
4. C4, in the same pull request as C3.

The commit order follows the list, so each red test turns green in its own commit. #1502 can merge before or after this plan, because C3 changes only the pack input (F2.7).

## 8. Verification

| Measure | Before (2026-11-16) | After |
|---|---|---|
| `ciparity` tests in the floor of a `required.yml`-only lane | 1 (pack) | every `ciparity` test that reads `.github/workflows/`, plus the pack test |
| `ciparity` tests at ship for the same lane | 1 | the same set as the floor |
| Repository-root readers with a data row | 0 of 66 candidates | every read that the C2 detector finds |
| Pack wall time at the floor, cold | about 30 s (F2.8) | 90 s or less (G4) |

Method:

- Run each red test before its component, and record the red line in the pull request body.
- Run `go test -count=1 ./internal/repocontract/... ./internal/phases/ship/... ./internal/core/...` after each component.
- Measure G4 with `evolve floor` on a worktree that changes one file under `docs/`, before and after C3.

## 9. Risks

| Risk | Control |
|---|---|
| The detector misses a read shape, for example `os.DirFS` or a read in a non-test helper package. | D6 makes an unresolved call with a root a read. The "must find" pin fails a blind detector. |
| A `show-toplevel` call serves a temporary repository, so the guard asks for a false row. | D6 reuses the root rule of the write scan, which treats only a call with no non-literal argument as a root (F2.12). |
| The data rows push the pack past its 120 s floor deadline, and the fail-open judge hides it. | D11 caps the count of by-name tests in a test, and D12 names the wall time in the WARN. If either limit is over, D5 changes to path selection by `Path`. Ship still runs the pack with no floor deadline. |
| A data-only lane fails on a row that was already red on main. | The pack reds go through the base attribution of the repo-contract floor (`go/internal/core/repo_contract_floor.go:54`). |
| `datafiles.go` grows past the 800-line file limit. | The rows hold prefixes, not files. If one package needs more than 40 names, a whole-package row is cheaper. |

## 10. Open questions

- **Q1.** What is the real cold time of the pack with the first rows? C2 gives the row count and sets `byNameTestCap`. After the build, measure the pack three times cold with `evolve floor` on a `docs/`-only worktree, and record the median in the package note. If the median is over 90 s, D5 changes.
- **Q2.** Does a data reader of `go/Makefile` stay a data row? The plan says yes: `go/Makefile` is outside the package directory of `ciparity`, and it is not a `.go` file (D8).

## 11. Review round 1 answers

| Finding | Answer in this revision |
|---|---|
| Architecture CRITICAL: a second by-name runner beside the pack | Design E, D4 and C3: the pack runs the rows by name. No new runner. |
| Architecture HIGH: the S6 split | F2.7: the pack runs in `productionSuiteFloorChecks`, the `Tests` entry of stage B. |
| Architecture HIGH: the guard is not fail-closed | D6, the "must find" pin in acceptance 2 and the cases of `TestDataReadProblems`. |
| Architecture MEDIUM: C5 and module-root files | C5 is out of scope (§3). It gets its own inbox item. |
| Architecture MEDIUM: an empty read key | D7 (revised in round 2). |
| Architecture LOW: the term "Lane floor" | The terms now define the build floor and its two test floors. |
| Docs HIGH: `floorTestPackages` pulls the rows into coverage | F2.2, design A rejected, non-goal in §3. |
| Docs HIGH: the importer backstop exits early and runs whole packages | F2.3 and F2.6: the rows run in `runFixedPack`, before the backstops. |
| Docs HIGH: PR #1502 | F2.7. |
| Docs MEDIUM: the `realtree_writes_test.go:97` citation | F2.12 cites `:353` and `:459`. |
| Docs MEDIUM: citations with no directory | Each code citation now has its full path. |
| Docs LOW: 122 against 143 | §2.4 gives the method: 328 files in 122 directories. The count of 143 did not reproduce. |

Round 2 findings:

| Finding | Answer in this revision |
|---|---|
| Architecture HIGH: D7 makes the two guards contradict over `TreeReadingTests()` | D7 now uses a data row keyed `WholeRepository` (`"/"`), and refuses only `""`. A `TestDataReadProblems` case proves that both guards are green on that row. |
| Architecture MEDIUM: the test names claim a path sensitivity that D5 removes | The acceptance tests are renamed to what they pin. `TestRunRepoContractPack_ADocsOnlyLaneGetsTheSameByNameCall` pins D5. §6 says how the criterion of the item is met. |
| Architecture MEDIUM: G4 is one measure, and the floor judge fails open | D11 caps the by-name count in a test. D12 names the wall time in the WARN. Q1 gives the measure to take after the build. |
| Architecture LOW: D9 puts tier policy into the registry | D9 filters in the ship consumer. `TestRepoContractSelections_LeaveOutEnvExclusiveRows` pins it, and C4 records the choice. |
| Architecture LOW: the log label of `runFixedPack` | C3 changes the label at `go/internal/phases/ship/repocontract.go:472`. |
