# Plan: a lane that changes a data file runs the tests that read that file

- **Status:** partly built, 2026-11-21 (4 of 8 components on main). Earlier status: proposed, 2026-11-16 (revision 5). The design review of revision 4 failed with 2 CRITICAL and 7 HIGH findings. The operator then chose a layered design (§1.1). Layer 1 is built now, and layers 2 to 4 are for later, in shadow.
- **Agent rule:** [ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md): an agent proposes, and the pipeline disposes. **Shadow stages:** ADR-0167, "shadow stages prove themselves on chained evidence" (proposed, branch `design/shadow-stage-contract`).
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
- **Always set:** the by-name tests that run on every pack run: the tree-reading rows, and the data rows keyed `WholeRepository` or under `go/` (D21).
- **Data set:** every other data row. It runs when the change holds a path that is not a `.go` file (D20).
- **Coarse set:** the always set plus the data set. Layer 1 runs it, with no precision for each file.
- **Narrow set:** the always set plus the data rows whose `Path` covers a changed path. Layer 4 computes it in shadow.
- **Monitor:** the LLM seat of layer 2. It proposes rows. It never decides what runs.

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
11. [Review answers](#11-review-answers)

## 1. Request

The inbox item (2026-11-16) says:

> A lane that changes only a non-Go data file (example: `.github/workflows/required.yml`) does not run the test packages that read that file (example: `go/internal/ciparity`).

The cause, in the words of the item: "the lane floor selects test packages from changed Go packages only".

### 1.1 The direction of the operator (2026-11-16)

After the review of revision 4, the operator asked: "Could we design a LLM monitor?"

The console answered with the layered plan of this revision. The operator replied:

> Ok

The layers:

1. **Layer 1 (build now):** a deterministic coarse rule. Any changed path that is not a `.go` file selects every known data row. It is fail-closed by over-running.
2. **Layer 2 (later):** an LLM monitor proposes rows, with `file:line` evidence.
3. **Layer 3 (later):** a deterministic check accepts or refuses each proposal before a row lands.
4. **Layer 4 (later):** the narrow set runs in shadow. It replaces the coarse set only after waves with zero escapes.

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
*Implication:* the tree-reading class has a working guard. Layer 1 does not change it.

**F2.11** The detector counts a climb only if it lands on the module root (`go/internal/repocontract/packages_test.go:186-197`). The case "past the module root" is a pinned negative (`go/internal/repocontract/packages_detector_test.go:24`). The package note says that such tests are "a different class: a changed non-Go file mapped to the tests that read it" (`docs/architecture/packages/internal-repocontract.md:40`).
*Implication:* the owner of the registry already named this gap and its shape.

**F2.12** `TestNoTestWritesIntoTheRealTree` follows a path root through helpers: a `..` climb, `os.Getwd()`, the file of `runtime.Caller` (`go/internal/repocontract/realtree_writes_test.go:353`) and `git rev-parse --show-toplevel` (`go/internal/repocontract/realtree_writes_test.go:459`). It fails if it finds no root helper, so a blind scan is red (`go/internal/repocontract/realtree_writes_test.go:13-17`). It does not resolve a method call on a root (`docs/architecture/packages/internal-repocontract.md:54`).
*Implication:* even this mature root analysis has shapes that it does not follow. Revisions 3 and 4 learned that a read detector inherits the same gap (§2.5).

**F2.13** `ciparity` finds the repository root through a helper, `workflowRepoRoot` (`go/internal/ciparity/workflows_test.go:190-197`). It joins `.github/workflows` and a file name, for example the constant `RequiredWorkflow` (`go/internal/ciparity/macos_weekly_test.go:182`, `go/internal/ciparity/ciparity.go:16`). `readWorkflowContract(t, name)` reads with a parameter tail (`go/internal/ciparity/workflows_test.go:177-179`).
*Implication:* a literal-only search misses most readers. The data rows of the build name `.github/workflows/` as a prefix for this reason.

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

*Implication:* the counts are an upper bound, because a `show-toplevel` call can serve a temporary repository.


### 2.5 Why revisions 3 and 4 failed

Revisions 3 and 4 tried to prove, before a test runs, which repository files the test reads. Each review round found a new Go shape that escaped the proof.

| Revision and round | The shape that escaped |
|---|---|
| Revision 3, build round 2 | A root as the receiver of a method (`datafiles_scan_test.go:490` on `fix/data-file-floor`). Keys relative to the module root, not the repository root. |
| Revision 3, build round 3 | A keyed composite literal (`datafiles_scan_test.go:325-347`). A surviving mutant of the `fmt.Sprintf` root rule (`:379-380`). |
| Revision 4, design review | `acsassert.RepoRoot` runs `--show-toplevel` in a package whose name does not end in `test` (`go/pkg/acsassert/assertions.go:189-196`). Seven packages call it. |
| Revision 4, design review | A climb split across statements, for example `filepath.Join(agentsDir, "..", "skills", …)` (`go/internal/phasecontract/self_review_test.go:81`), in 41 files. |
| Revision 4, design review | Spelling variants: an import alias, another `runtime` function, other `git` flags. |
| Revision 4, design review | A data file inside the module, for example `go/Makefile`, is not past the module root, so the climb rule did not apply. |

**F2.14** A test can build a path from any value that it computes at run time. A read of a given file is a semantic property of the program. [Rice's theorem](https://en.wikipedia.org/wiki/Rice%27s_theorem) says that no static check decides every non-trivial semantic property.
*Implication:* a static detector or a syntactic guard of reads is open by construction. More review rounds find more shapes, and they do not converge.

**F2.15** The build of revision 3 holds 93 data rows in 40 packages, with 539 test names (`go/internal/repocontract/datafiles.go`, uncommitted in the `dev/cl-floorbuild` worktree of `fix/data-file-floor`). It also holds `WholeRepository`, `ByNameTests()`, the pack wiring, the cap test and the env-exclusive filter in the ship consumer. The console measured the whole by-name set, 646 tests, at about 74 s warm (2026-11-16).
*Implication:* the registry and its runner work. Only the proof that every reader has a row failed.

**F2.16** Real tests read Markdown files under `docs/`. Rows of the build name `docs/operations/runtime-reference.md`, `docs/architecture/signal-codes.md`, `docs/architecture/review-loop-and-quality-index.md` and `docs/architecture/packages/internal-gc.md`. Twenty rows are keyed `WholeRepository`, for example the real-tree test of `internal/stelint`. Rows also name `.md` files under `agents/` and `skills/`.
*Implication:* a rule that leaves out `docs/**/*.md` skips tests that a docs edit can break. Layer 1 leaves out no path class (D20).

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| G1. A data-only lane runs the data readers in its build floor. | `TestRunRepoContractPack_AWorkflowOnlyChangeRunsTheTestsThatReadIt` | red before C3, green after |
| G2. The ship gate runs the same data readers, also when no Go file changed. | `TestRunRepoContractGate_AWorkflowOnlyChangeRunsTheTestsThatReadIt` | red before C3, green after |
| G3. A Go-only lane does not pay for the data set. | `TestRunRepoContractPack_AGoOnlyChangeLeavesOutTheDataSet` | the data set is absent from the call |
| G4. The pack stays inside its floor deadline. | pack wall time at the floor, and the count of by-name tests | 90 s or less, against the 120 s deadline (F2.8); the count under `byNameTestCap` |
| G5. Layers 2 to 4 narrow the coarse set with no lost red. | escapes recorded in shadow (layer 4) | 0 escapes in N waves before the flip |

Non-goals of layer 1:

- Layer 1 does not prove that every reader has a row. No detector and no syntactic guard of reads exists (F2.14).
- Layer 1 does not select rows by each changed file. It selects the whole data set or none of it.
- The plan does not change CI routing in `.github/workflows/required.yml`. CI stays the last backstop.
- The plan does not select `go/acs` predicates. The audit ACS run owns them.
- The plan does not change the unit-test floor, `floorTestPackages` or the cover-changed floor (F2.2).

**The limit of layer 1.** A new test that reads a data file and has no row is not in the coarse set. A later lane that changes that data file does not run it. The lane that adds the test runs it in its own unit-test floor, because it changes the package. The answers to this limit are layer 2 and the ordinary review.

The second acceptance criterion of the item ("a guard fails when a test package reads a repo data file that no registry row names") is replaced by layers 2 to 4. The operator approved this direction (§1.1).

## 4. Candidate designs

Where the rows run (decided in revision 2, kept):

| Design | Catches `required.yml` → `ciparity` | Cost for a lane | One runner and one home | Score |
|---|---|---|---|---|
| A. Unit-test floor path (`floorTestPackages`) | 4 | 4 | 1 | 9 |
| E. The pack runs the rows by name | 5 | 3 | 5 | 13 |

How a change selects the rows (revision 5):

- **S1. Proof of every read.** A static detector or guard proves a row for each reader. Revisions 3 and 4 (§2.5).
- **S2. Always run every row.** The build of revision 3 did this (old D5).
- **S3. Coarse trigger.** A change that holds a non-`.go` path runs every data row. A Go-only change runs the always set.
- **S4. Exact path match.** Each changed path runs the rows whose `Path` covers it.

| Design | Fail-closed for known rows | Cost for a Go-only lane | Build cost | Depends on row precision | Score |
|---|---|---|---|---|---|
| S1. Proof of every read | 1 | 4 | 1 | 5 | 11 |
| S2. Always run every row | 5 | 1 | 5 | 5 | 16 |
| S3. Coarse trigger | 5 | 4 | 4 | 5 | 18 |
| S4. Exact path match | 2 | 5 | 4 | 1 | 12 |

Each criterion has a score from 1 (bad) to 5 (good). S3 wins because it keeps the over-run of S2 for every data change and saves the data set for Go-only lanes. S4 trusts that each `Path` is exact, and no check proves that today. Layer 4 earns S4 with shadow evidence.

## 5. Decisions

Layer 1 (build now):

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Keep the rows of the build: `DataFileRead{Path; TestSelection}`, `WholeRepository`, `ByNameTests()`, in `go/internal/repocontract/datafiles.go`. | F2.15 | A new registry: the current one works. |
| D3 | A row names tests, as `treeReadingSelections` does. | F2.9 | Whole packages: a lane then runs all of `cmd/evolve`. |
| D4 | The pack runs the selections by name in `defaultRepoContractTest`, at the floor (stage B, `productionSuiteFloorChecks`) and at ship (`runFixedPack`). | F2.5, F2.6, F2.7 | A second by-name runner: two `-run` builders and two retake rules. |
| D9 | The ship consumer leaves out the rows of env-exclusive packages. | F2.4 | A filter in the registry: an edge from the registry to the tier policy. |
| D11 | `TestByNameTests_StaysUnderTheCap` caps the by-name count at `byNameTestCap`. | The floor judge fails open on a timeout (`go/internal/core/repo_contract_floor.go:40-41`). | A wall-time measure only: it does not stop growth. |
| D12 | The WARN of an unnamed pack exit names the wall time and the deadline. | The same fail-open path. | No change: a timeout stays silent about its cause. |
| D19 | Remove the read detector and the row guard of the build (`datafiles_scan_test.go`, `datafiles_shapes_test.go` and the "every reader has a row" test). | F2.14 | A further patch round: §2.5. |
| D20 | The data set runs when the change holds any path that is not a `.go` file. No path class is left out, `docs/` included. | F2.16 | Leaving out `docs/**/*.md`: real tests read those files. |
| D21 | The always set holds the tree-reading rows and the data rows keyed `WholeRepository` or under `go/`. It runs on every pack run. | A `.go` change can break a test that reads the tree or a file under `go/`. | The data set only: a Go-only lane then skips `WholeRepository` rows. |
| D22 | `repocontract.ByNameTestsFor(changed []string)` returns the always set, plus the data set when D20 applies. `ByNameTests()` stays the coarse set. | One selector for the floor and ship. | Two selectors: they drift. |
| D23 | `RepoContractPackFn` gets the changed paths. The floor passes `floorPaths`. If the floor cannot read the paths, the pack runs `ByNameTests()`. At ship, `runRepoContractSteps` calls `changedFilesTwice` one time, before `runFixedPack`, and gives the files to the pack and to both backstops. A discovery failure at ship stays an INFRA refusal, so ship has no fallback. `repoContractTestFn` and the log line of `runFixedPack` take the set from `ByNameTestsFor`. | F2.6: the pack has no paths today. The floor fails closed by over-running, and ship fails closed by refusing. | A pack that reads git itself, or a second discovery before the pack: two ways to compute one change. A ship fallback: the discovery error stops ship first, so the fallback is dead code. |
| D24 | `TestDataFileReads_EveryNamedTestExists` fails when a row names a test function that its package does not declare. `TestDataFileReads_EveryPathExists` fails when a row `Path` is absent from the tree. | `go test -run` with an absent name runs nothing and passes. | No check: a rename silently empties a row. |

D2, D5, D6, D7, D8, D10 and D13 to D18 of earlier revisions are retired. D2 (the `Path` kinds) and D7 (no `""` path) stay as the kind rule of the build.

Layers 2 to 4 (design only):

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D25 | Layer 2: a read-only seat, `data-read-monitor`, runs through `evolve agent run`. It reads the changed `_test.go` files and changed data files of a lane. It writes proposals: data path, package, test names and `file:line` evidence. | ADR-0155: an agent proposes. The seat runs through the bridge like each other seat. | A seat that writes rows: an agent then disposes. |
| D26 | Layer 3: a deterministic check accepts a proposal on one condition: the test fails when the file is absent. The check moves the file away in a scratch copy of the worktree, and runs the test there. The `file:line` evidence is a record for the review, not a condition. For a prefix row or a `WholeRepository` row, the check moves away the files that the proposal names under that prefix. | ADR-0155: the pipeline disposes. A row that does not change a result is noise. F2.13: most readers build the path from a helper or a parameter, so a literal search refuses real readers. | A literal path in the test source as a condition: it refuses the helper readers of F2.13. The monitor edits `datafiles.go` itself: an agent then disposes. |
| D27 | An accepted row lands through an ordinary pull request with its check output. A refused proposal is logged with its reason, and nothing lands. | The rows are code, so the ship gate and review cover them. | A direct write of accepted rows: no review covers them. |
| D28 | Layer 4: in shadow, the pack computes the narrow set and runs the coarse set. An escape is a failed test outside the narrow set that the base attribution of the floor (`baseattr`) charges to the lane. A test that is red on the base, or a flake that a retake clears, is not an escape. Each escape is recorded under the shadow-stage contract of ADR-0167. | A flip needs chained evidence, not a claim. Noise must not reset the count of D29. | Each failure outside the narrow set as an escape: the flip stalls on base reds and flakes. |
| D29 | The flip from coarse to narrow needs N consecutive waves with zero escapes, and a built enforce path. The operator sets N. | ADR-0167 and the rule that a shadow enables only with a built enforce path. | A flip on a claim, or on a fixed N in code. |
| D30 | The guarantee never depends on the monitor. If layer 2 stops, layer 1 still runs the coarse set. | ADR-0155. | A narrow set with no coarse fallback. |

We revisit D20 if layer 4 shows zero escapes for `docs/` paths over N waves. We revisit D21 if the always set passes half of `byNameTestCap`.

Consequences:

- Good: a data-only lane gets the same red as main CI, at the floor and at ship, before it merges.
- Good: a Go-only lane runs only the always set, so it pays less than the build of revision 3.
- Cost: a lane with any non-Go path runs the coarse set, about 74 s warm (F2.15).
- Limit: a reader with no row is not selected (§3). Layer 2 and review answer it.

## 6. Components

Layer 1:

| Id | Component | Red test first | Owner |
|---|---|---|---|
| C1 | `ByNameTestsFor(changed)` in `go/internal/repocontract/datafiles.go` (D20 to D22). | `TestByNameTestsFor_ANonGoPathSelectsTheCoarseSet`, `TestByNameTestsFor_ADocsOnlyChangeSelectsTheCoarseSet` and `TestByNameTestsFor_AGoOnlyChangeSelectsTheAlwaysSet` | console |
| C2 | Remove the detector and the row guard (D19). Add the two row checks (D24). | `TestDataFileReads_EveryNamedTestExists` and `TestDataFileReads_EveryPathExists` | console |
| C3 | `RepoContractPackFn` takes the changed paths (D23). The floor judge and `runFixedPack` pass them. The ship consumer keeps the env-exclusive filter (D9). | `TestRunRepoContractPack_AWorkflowOnlyChangeRunsTheTestsThatReadIt`, `TestRunRepoContractPack_AGoOnlyChangeLeavesOutTheDataSet`, `TestRunRepoContractGate_AWorkflowOnlyChangeRunsTheTestsThatReadIt` and `TestRepoContractFloorJudge_AnUnreadableChangeRunsTheCoarseSet` and `TestRepoContractFloorJudge_AWorkflowOnlyWorktreeRunsTheTestsThatReadIt` | console |
| C4 | Package notes: replace the skip line at `docs/architecture/packages/internal-repocontract.md:40`. Record D20, D21, D23, the limit of §3 and `byNameTestCap`. | `evolve docs ste-lint --strict` and the link check | console |

Layers 2 to 4 (later, each after its own design review):

| Id | Component | Red test first | Owner |
|---|---|---|---|
| C5 | The `data-read-monitor` persona and its proposal schema (D25). | `TestDataReadProposal_RefusesAProposalWithNoEvidenceLine` | console |
| C6 | The acceptance check (D26, D27). | `TestAcceptProposal_RefusesATestThatPassesWithTheFileAbsent` | console |
| C7 | The shadow narrow set and the escape record (D28). | `TestNarrowShadow_AFailureOutsideTheNarrowSetIsAnEscape` | console |
| C8 | The enforce path and the flip (D29). | `TestNarrowFlip_RefusedBeforeNWavesWithZeroEscapes` | console |

The acceptance tests of layer 1:

- **Acceptance 1:** `TestRunRepoContractPack_AWorkflowOnlyChangeRunsTheTestsThatReadIt` in `go/internal/phases/ship`. `RunRepoContractPack` is the function that the real floor entry calls (F2.5, F2.7). The changed paths are `[".github/workflows/required.yml"]`. The test records the `go test` arguments through `goTestJSONFn` (`go/internal/phases/ship/repocontract.go:166`). It asserts `./internal/ciparity` and `TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies` in the by-name call. On `origin/main`, the call names only `TestEnvExclusive_TheCIJobsTheBridgeBackstopNamesRunTheTmuxTier`.
- **The Go-only pin:** `TestRunRepoContractPack_AGoOnlyChangeLeavesOutTheDataSet` uses the paths `["go/internal/core/x.go"]`. It asserts the always set, and no `ciparity` workflow test.
- **Ship side:** `TestRunRepoContractGate_AWorkflowOnlyChangeRunsTheTestsThatReadIt` drives `runRepoContractGateAt` on a worktree that changes only `required.yml`. It asserts the same by-name call, and the importer backstop line "no Go change in the tree — skipped".
- **Fail-closed pin:** `TestRepoContractFloorJudge_AnUnreadableChangeRunsTheCoarseSet` gives an error for the changed paths. It asserts the call of `ByNameTests()`.
- **Floor wiring pin:** `TestRepoContractFloorJudge_AWorkflowOnlyWorktreeRunsTheTestsThatReadIt` drives `RepoContractFloorJudge` on a temporary git worktree that changes only `.github/workflows/required.yml`, with a recording `RepoContractPackFn`. It asserts that the recorded paths hold that file, and that the selection holds `TestRequiredWorkflow_AlwaysReportsAndCannotSkipDependencies`. It is red on `origin/main`, because `RepoContractPackFn` has no paths parameter there. Without it, a judge that passes no paths runs only the always set, and each other test stays green.

## 7. Order

1. C2, because the build branch must lose its detector before the selector changes.
2. C1, because C3 calls the selector.
3. C3, because it closes acceptance 1 and the ship side.
4. C4, in the same pull request as C3.
5. C5 to C8, later, each after a design review. C7 runs in shadow for N waves before C8.

#1502 can merge before or after layer 1, because the pack stays in stage B (F2.7).

## 8. Verification

| Measure | Before (2026-11-16) | After layer 1 |
|---|---|---|
| `ciparity` tests in the floor of a `required.yml`-only lane | 1 | every `ciparity` test in a data row, plus the pack test |
| `ciparity` tests at ship for the same lane | 1 | the same set as the floor |
| By-name tests for a Go-only lane | 143 on `origin/main`; 646 on the build of revision 3 | the always set (measured at C1) |
| By-name tests for a lane with a non-Go path | 143 | the coarse set, 646 on the build |
| Pack wall time, coarse set | about 74 s warm (F2.15) | 90 s or less cold (G4) |

Method:

- Run each red test before its component, and record the red line in the pull request body.
- Run `go test -count=1 ./internal/repocontract/... ./internal/phases/ship/... ./internal/core/...` after each component.
- Measure the pack three times cold with `evolve floor` on a worktree that changes one file under `docs/`, and record the median in the package note.

## 9. Risks

| Risk | Control |
|---|---|
| A new data reader has no row, so a later data lane does not run it. | The adding lane runs it in its own unit-test floor. Layer 2 proposes the row. Review asks for it. CI on main is the last backstop. |
| The coarse set passes the 120 s floor deadline cold, and the fail-open judge hides it. | D11 caps the count. D12 names the wall time in the WARN. Ship runs the pack with no floor deadline. |
| A row names a test that was renamed, so it runs nothing. | D24 fails on an absent test name or an absent `Path`. |
| The monitor proposes a false row. | D26 refuses a row whose test passes with the file absent. |
| The monitor stops or degrades. | D30: layer 1 does not depend on it. |
| A narrow set loses a red after the flip. | D28 and D29: the flip needs N waves with zero escapes, on chained evidence. |

## 10. Open questions

- **Q1.** What is the cold time of the coarse set? If the median is over 90 s, the always set and the data set need a split by cost, or layer 4 must come first.
- **Q2.** What is N for the flip of D29? The operator sets it. The plan proposes 10 waves.
- **Q3.** Where does the monitor run: at each lane handoff, or once at each boundary over the merged diffs? A boundary run costs less. A lane run finds the row before the next data lane.

## 11. Review answers

Revision 4 design review (2 CRITICAL, 7 HIGH):

| Finding | Answer in revision 5 |
|---|---|
| CRITICAL 1: the scan scope misses `acsassert.RepoRoot` and production root wrappers | Non-goal of layer 1: no scan exists (D19). Layer 1 selects by changed paths, not by reads. |
| CRITICAL 2: cross-package root wrappers bypass the credit of each package | Non-goal: no credit analysis exists (D19). |
| HIGH 3: a climb split across statements escapes rules 3 and 4 | Non-goal: no climb rule exists. F2.14 records why. |
| HIGH 4: spelling variants escape, and the "must find" pin was dropped | Non-goal: no match by spelling exists. |
| HIGH 5: the baseline counts shapes, not readers | Retired with the baseline. |
| HIGH 6: the write guard goes blind as the migration proceeds | Retired: no migration to the helper takes place, and the write guard is not changed. |
| Docs HIGH: `TestRoot_IsTheDirectoryThatHoldsGoMod` names the module root | Retired with the helper. |
| Docs HIGH: rule 3 fails open for `go/Makefile` inside the module | D20: `go/Makefile` is not a `.go` file, so it runs the data set. |
| Docs HIGH: the syntactic guard is not fail-closed | Agreed (F2.14). Layer 1 is fail-closed by over-running instead. |

Build of revision 3, rounds 1 to 3: §2.5 records each finding. D19 removes the code that held them.

Design review rounds 1 and 2: D1 to D12 keep their answers. These are the pack seam, the S6 split, `WholeRepository`, the cap and the env-exclusive filter in the ship consumer.

Revision 5 design review (0 CRITICAL, 3 HIGH):

| Finding | Answer |
|---|---|
| Arch HIGH 1: the floor judge's use of the changed paths has no wiring test | Agreed. C3 adds `TestRepoContractFloorJudge_AWorkflowOnlyWorktreeRunsTheTestsThatReadIt` (the floor wiring pin). |
| Arch HIGH 2: D23 does not say how ship gets the paths | Agreed. D23 now calls `changedFilesTwice` one time before `runFixedPack`. A discovery failure at ship stays an INFRA refusal, and the fallback is for the floor only. |
| Docs HIGH and Arch MEDIUM 3: the literal path of D26 refuses the readers of F2.13 | Agreed. D26 keeps one condition: the test fails with the file absent. The evidence is a record only. |
| Arch MEDIUM 4: D28 counts noise as an escape | Agreed. D28 counts only a failure that `baseattr` charges to the lane. |
| Docs MEDIUM: no rejected options for D25 to D30, no owner for C5 to C8 | Agreed. Both columns are added. |
| Docs LOW: 94 rows | Corrected to 93. The build is uncommitted in `dev/cl-floorbuild`. |

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| C1 `ByNameTestsFor` | done, 2026-11-18 | [#1538](https://github.com/mickeyyaya/evolve-loop/pull/1538) [`c2a6cce`](https://github.com/mickeyyaya/evolve-loop/commit/c2a6cce2f) (`go/internal/repocontract/datafiles.go:31`) | none |
| C2 remove detector, add the two row checks | done, 2026-11-18 | [#1538](https://github.com/mickeyyaya/evolve-loop/pull/1538) [`c2a6cce`](https://github.com/mickeyyaya/evolve-loop/commit/c2a6cce2f) (`go/internal/repocontract/datafiles_rows_test.go`) | none |
| C3 `RepoContractPackFn` takes the changed paths | done, 2026-11-18 | [#1538](https://github.com/mickeyyaya/evolve-loop/pull/1538) [`c2a6cce`](https://github.com/mickeyyaya/evolve-loop/commit/c2a6cce2f) (`go/internal/phases/ship/repocontract_datafiles_test.go`, `go/internal/core/repo_contract_floor_test.go`) | none |
| C4 package notes | done, 2026-11-18 | [#1538](https://github.com/mickeyyaya/evolve-loop/pull/1538) [`c2a6cce`](https://github.com/mickeyyaya/evolve-loop/commit/c2a6cce2f); the skip line at `internal-repocontract.md:40` is gone | none |
| C5 `data-read-monitor` persona | not started | none | none (draft: data-file-floor-c5-monitor-persona) |
| C6 acceptance check | not started | none | none (draft: data-file-floor-c6-acceptance-check) |
| C7 shadow narrow set and escape record | not started | none | none (draft: data-file-floor-c7-narrow-shadow) |
| C8 enforce path and flip | not started | none | none (draft: data-file-floor-c8-narrow-flip) |

Layer 1 is on main in PR 1538. The inbox item `lane-floor-selects-the-tests-that-read-a-changed-data-file` is still open, although layer 1 closes it. No commit carries its `Closes-Inbox` trailer, so the item needs a consume. The related item `floor-runs-the-package-of-a-changed-testdata-file` is built in the open PR [#1626](https://github.com/mickeyyaya/evolve-loop/pull/1626).

Layers 2 to 4 have no code and no inbox item. I wrote one follow-up draft for each. C7 and C8 also depend on the shadow-stage contract of ADR-0167, which is partly built.
