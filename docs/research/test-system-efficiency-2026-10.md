# The test system: efficiency, effect and scale (research, 2026-10)

> The plan that uses this research: [test-system-efficiency-2026-10.md](../plans/test-system-efficiency-2026-10.md). The design: [test-system.md](../architecture/test-system.md). The decision record: [ADR-0131](../architecture/adr/0131-one-test-run-feeds-every-coverage-gate.md).
>
> The earlier campaign: [test-refactoring-design-2026-09-14.md](../architecture/test-refactoring-design-2026-09-14.md) (gate honesty, fake fidelity, safe consolidation). This dossier does not repeat it. It adds the cost of the suite, its growth and the sources of new duplication.

Date: 2026-10-12. Method: measurement on `main` at `be0a0a979`, a static scan of the test files, and a survey of current practice. Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

1. [Inventory and growth](#1-inventory-and-growth)
2. [The flow of the floor and of CI](#2-the-flow-of-the-floor-and-of-ci)
3. [Wall time and its poles](#3-wall-time-and-its-poles)
4. [Duplication](#4-duplication)
5. [Isolation and parallel tests](#5-isolation-and-parallel-tests)
6. [Current practice](#6-current-practice)
7. [Adopt or reject](#7-adopt-or-reject)
8. [Refinements to adopt](#8-refinements-to-adopt)
9. [The timeout investigation](#9-the-timeout-investigation)

## 1. Inventory and growth

**F1.1** The module has 4,007 test files and 20,535 `Test` functions. It also has 11 fuzz targets and 8 benchmarks (`git grep '^func Test'`).
*Implication:* the suite is large. A cost that is small for one test is large for the suite.

**F1.2** The runtime part (all files outside `acs/`) has 3,403 test files and 16,777 `Test` functions. The 533 directories under `acs/` hold per-cycle predicates. Only `acs/regression` (42 directories) runs in CI.

**F1.3** On 2026-09-14 the suite had 2,902 test files and 14,496 `Test` functions ([test-refactoring-design-2026-09-14.md](../architecture/test-refactoring-design-2026-09-14.md)). In 4 weeks, 1,274 test files were added, and the net growth was 115,901 test lines (`git log --numstat`).
*Implication:* the suite grew by 42% in 4 weeks. A cost that grows with the suite grows fast.

**F1.4** Test code has 563,872 lines. Production code has 217,264 lines. The ratio is 2.6 to 1.

**F1.5** The rule "each changed function at 100%" ([cover-changed-gate-2026-10.md](../plans/cover-changed-gate-2026-10.md)) and the backfill lanes add tests at a high rate. On 2026-10-12, 774 functions that changed in 14 days were still below 100% (`evolve cover changed --base 15b3b1123`).
*Implication:* the growth continues. The design must keep the cost per test low and must stop new duplication at the source.

## 2. The flow of the floor and of CI

**F2.1** No test file has a `!integration` build constraint (`git grep -l '^//go:build.*!integration'`: 0 files). Thus `make test` runs a strict subset of the tests that `make test-integration` runs.

**F2.2** `make apicover-enforce` runs `go test -tags integration -coverprofile=coverage.apicover.txt` again over the packages in `go/.apicover-enforce` (`go/Makefile:161`). These are 268 of the 285 runtime packages. They hold 2,644 of the 2,953 package-seconds of the integration run.

**F2.3** `make cover-strict` runs `go test -cover` once more for each of the 51 packages in `go/.cover-strict`, one package at a time (`go/Makefile:143`). CI runs the same target (`.github/workflows/go.yml:130`).

**F2.4** CI already reuses the profile for the API gate: `make apicover-check` reads the `coverage.txt` of the integration step (`go.yml:116`). `make cover-changed` reads it too (`go.yml:62`).

**F2.5** The floor of lane D (2026-10-12, four lanes on the host) took these times, from the log file times in the lane scratchpad:

| Target | Minutes | Runs again what `test-integration` ran |
|---|---|---|
| `test` | 6.7 | yes, a strict subset |
| `test-integration` | 7.4 | (the one run that is necessary) |
| `test-e2e` | 2.7 | no |
| `test-acs-durable` | 0.4 | no |
| `apicover-enforce` | 5.6 | yes, 268 of 285 packages |
| `cover-strict` | 0.6 | yes, 51 packages, one at a time |

*Implication:* 12.9 of 23.4 minutes (55%) of the local floor runs tests a second or third time. CI pays the repeat only for `cover-strict`.

**F2.6** `go tool covdata percent` gives per-package percents that "correspond directly to those reported by `go test -cover`" ([cmd/covdata](https://pkg.go.dev/cmd/covdata)). The text profile of `go test -coverprofile` holds the same blocks and counts. `internal/covchanged.ParseProfile` already parses it and merges duplicate blocks (`go/internal/covchanged/profile.go:79`).
*Implication:* a per-package percent from `coverage.txt` gives the same number as a second `go test -cover` run. No second run is necessary to judge a floor.

**F2.7** `go test -coverprofile` writes the profile also when a test fails (synthesis). The 2026-09-14 S1 slice made the gates fail on a failed test. A reader of the profile cannot see if the run passed.
*Implication:* a gate that only reads the profile needs a proof that the run passed. The run must write the profile under its final name only on success.

## 3. Wall time and its poles

**F3.1** One `make test-integration` run (2026-10-12, `-race`) spent 2,953 package-seconds over 285 packages. 7 packages took 60 s or more. The 10 slowest packages held 60% of the time.

| Package | Seconds |
|---|---|
| `internal/core` | 352 |
| `internal/subagent` | 295 |
| `internal/phases/ship` | 287 |
| `cmd/evolve` | 230 |
| `internal/bridge` | 210 |
| `internal/phases/audit` | 173 |
| `internal/ciparity` | 75 |

**F3.2** `go test` runs packages at the same time. Thus the wall time of the run is close to the build time plus the time of the slowest packages. The many small packages finish early. The poles set the end.

**F3.3** The time in a pole is in few tests. Data from `go test -json` on 2026-10-12, with four lane floors on the host:

| Package | Top-level tests | Tests of 1 s or more | Their part of the time |
|---|---|---|---|
| `internal/subagent` | 264 | 1 | 99.6% (479 s in one test) |
| `internal/phases/audit` | 331 | 22 | 97% |
| `internal/ciparity` | 50 | 17 | 99% |
| `internal/bridge` | 1,185 | 62 | 83% |
| `internal/phases/ship` | 606 | 180 | 88% |
| `cmd/evolve` | 1,698 | 120 | 78% |
| `internal/core` | 1,355 | 156 | 74% |

*Implication:* a fix to a small number of tests moves the wall time of the run. A broad rewrite is not necessary.

**F3.4** `TestDefaultExecAdapter_DispatchesViaBridge` (`go/internal/subagent/validateprofile_test.go`) took 479 s. A stack dump at 40 s shows `bridge.(*Engine).Launch` → `recordModelAttempt` → `tokenusage.ScanConfigRoot` → `readLines` (`go/internal/tokenusage/scanner.go:82`). The scan reads the real `~/.claude/projects` of the host: 3,799 files, 10 GB.
*Implication:* the test is not hermetic. Its time grows with the use of Claude on the host. In CI, `~/.claude` is empty, so CI does not see this pole.

**F3.5** Production uses the same scan. `internal/adapters/bridge/bridge.go:110` wires `tokenusage.DefaultResolver(tokenusage.ClaudeConfigRoot(env))`. Thus each dispatch on a Claude driver reads the full transcript store after it ends.

**F3.6** The scan counts only assistant lines with a timestamp inside `[Start, End]` (`scanner.go:138`, `withinWindow`). Claude appends lines to a transcript while the session runs. A file that changed last before `Start` cannot hold a line that counts. On 2026-10-12, a window of 60 minutes matched 9 files (250 MB) of 3,799 (10 GB).
*Implication:* a filter on the file modification time skips about 97% of the bytes and does not change any result.

**F3.7** Under load, `internal/core` passed the 10-minute default timeout of `go test` (604 s, "test timed out after 10m0s"). `make test-integration` sets no `-timeout`. On a host with less load, the same package took 352 s.
*Implication:* the largest package uses about 60% of the default timeout. With the growth of F1.3, CI will reach the limit.

**F3.8** The slow tests of `internal/phases/audit` are the skills-drift gate tests (`audit_skillsdrift_closure_integration_test.go`, `audit_skillsdrift_worktree_integration_test.go`). Each test builds a lane repository and runs the generator of the worktree. 9 of them take from 17 to 74 s each, 296 s in all. None of them calls `t.Parallel`, `t.Setenv` or `t.Chdir`.
*Implication:* these tests can run in parallel. Their time then overlaps.

**F3.9** The slow tests of `internal/ciparity` run real `make` recipes with a real `go test` inside (`TestMakeCoverageGates_PropagateTestFailure`, 36 s). These tests protect the gates. Their cost is correct, but they also run one at a time.

**F3.10** 79 calls to `time.Sleep` are in runtime tests. Most are in `cmd/evolve` (14), `internal/adapters/observer` (12), `internal/bridge` (11) and `internal/bridge/channel` (8).

## 4. Duplication

**F4.1** An AST scan normalized the body of each non-test function in each test file (`go/printer`, names removed) and grouped equal bodies. It found 346 groups with 858 extra copies and about 8,000 extra lines.

**F4.2** Of the 858 extra copies, 597 (5,757 lines) are in `acs/` predicates. The largest groups:

| Copies | Helper names | Lines each |
|---|---|---|
| 107 | `goDir`, `goModuleDir`, `moduleRoot` and 3 more | 4 |
| 28 + 16 | `runGoTest` (two variants) | 9 and 12 |
| 26 + 21 | `tail`, `tailLines` | 7 |
| 16 | `walkModule` | 8 |
| 13 | `requireTestsRan` | 10 |
| 11 | `execGo`, `funcCoverage` | 13 |
| 5 | `assertMutantsKilled` | 43 |

*Implication:* each cycle writes its own copy of the same predicate helpers. `pkg/acsassert` has file and JSON checks, but it has no helper to find the module, to run `go test` or to read a tail.

**F4.3** The runtime part has 261 extra copies. The largest groups are file writers (24 copies with 10 names) and slice compares (11 copies with 6 names). Many packages also define `contains` (15) and `itoa` (11). The standard library has these functions: `os.WriteFile`, `slices.Equal`, `strings.Contains`, `slices.Contains` and `strconv.Itoa`.

**F4.4** The kits exist and are in use. The count of importers: `go/test/fixtures` 108, `internal/gittest` 115, `internal/fakeclitest` 49, `internal/routingtest` 8 and `internal/cliroute/cliroutetest` 8. Also `internal/kerneltest` 7, `internal/testhost` 7 and `internal/tmuxtest` 4. `go/docs/testing.md` names `fixtures` the single source of truth.

**F4.5** 204 runtime test files run `git` directly with `exec.Command`. 115 files use `internal/gittest`. `internal/rawgitratchet` refuses only a new raw `git init` fixture.

**F4.6** On 2026-10-12, a backfill lane added a package-level seam with the generic name `processAbs` in `cmd/evolve`. Two other lanes added files to the same package at the same time. Two equal top-level names in one package break the build after both merges, and each pull request is green alone.
*Implication:* parallel test lanes need a rule for names, and a check before the merge train.

## 5. Isolation and parallel tests

**F5.1** 2,791 calls to `t.Parallel` are in the suite, for 20,535 tests (13.6%). `internal/core` has 613 for 2,204 tests. `internal/phases/ship` has 6 for 669 tests.

**F5.2** 631 calls to `t.Setenv` or `os.Setenv` are in 233 runtime test files. 83 calls to `os.Chdir` or `t.Chdir` exist. `cmd/evolve` alone has 219 `Setenv` calls.

**F5.3** `t.Setenv` and `t.Chdir` mark a test as one that cannot run in parallel. A call to `t.Parallel` in such a test panics, in either order ([alexwlchan.net](https://alexwlchan.net/notes/2026/serial-tests-in-go/), [golang/go#78614](https://github.com/golang/go/issues/78614)).

**F5.4** 174 tests restore a package-level variable in `t.Cleanup` (`t.Cleanup(func() { x = orig })`). A test that changes a package variable cannot run in parallel with a test that reads it. The race detector reports the conflict.

**F5.5** The house rule for an unreachable guard is "a package-variable seam bound to the exact original" ([feedback memory, LQ-12]; the backfill brief). Each such seam adds one test that must run alone.
*Implication:* a seam in a dependency struct, or a parameter, keeps the test parallel. A package variable is the fallback, for a function with no dependency struct.

## 6. Current practice

**P6.1 Coverage from one run.** Go reports per-package coverage from the profile data, with the same numbers as `go test -cover` ([cmd/covdata](https://pkg.go.dev/cmd/covdata)). Build systems with hermetic tests keep one instrumented run and give its output to each report (synthesis).

**P6.2 Fake time.** `testing/synctest` became stable in Go 1.25. In a bubble, `time.Sleep` and timers use a fake clock. The clock moves when all goroutines of the bubble block, so a long sleep costs no real time ([pkg.go.dev/testing/synctest](https://pkg.go.dev/testing/synctest), [Go 1.25 tour](https://antonz.org/go-1-25/)). A test in a bubble cannot call `T.Run`, `T.Parallel` or `T.Deadline`.

**P6.3 Serial markers.** `t.Setenv` and `t.Chdir` set a flag that refuses `t.Parallel` (F5.3). A proposal asks for a `t.Serial()` method. It is not part of the released API ([golang/go#78614](https://github.com/golang/go/issues/78614)).

**P6.4 Test selection at scale.** At Google, TAP ran 150 million tests in one day. Only 1.23% of the test runs found a break or a fix. The compute grew with the square of the commit rate, and this growth caused the move to selection ([Memon and others, ICSE SEIP 2017](https://huang.isis.vanderbilt.edu/cs8395-f23/paper/google-testing-icse-seip-17.pdf)). A Google study found that selection by recent pass/fail transitions did less well than expected ([research.google](https://research.google/pubs/assessing-transition-based-test-selection-algorithms-at-google/)).

**P6.5 Predictive selection.** Meta selects tests with a learned model ("Predictive Test Selection", 2019). Vendor summaries quote 99.9% of regressions caught with 33% of the tests. We did not verify these numbers in the paper ([BrowserStack summary](https://www.browserstack.com/guide/predictive-test-selection), [TU Wien replication, 2025](https://repositum.tuwien.at/bitstream/20.500.12708/215633/1/Aichmann%20Stefan%20-%202025%20-%20Predictive%20Test%20Selection%20A%20Replication%20Study.pdf)). Static test impact analysis maps changes to tests by dependencies ([Launchable](https://launchableinc.com/what-is-predictive-test-selection)).

**P6.6 The local graph.** `internal/changedpkgs` already computes the packages that a change touches and their importer closure (`ImporterClosure`, `go/internal/changedpkgs/importerclosure.go:27`). The ship phase and the audit phase use it.

## 7. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| One instrumented run, many readers (P6.1, F2.6) | Adopt | It removes 55% of the floor time (F2.5) and gives one source of truth for every coverage number. |
| A profile written only on success (F2.7) | Adopt | A reader needs a proof that the run passed. |
| Filter the transcript scan by file time (F3.6) | Adopt | Same result, about 97% fewer bytes, in tests and in production. |
| Hermetic home in tests (F3.4) | Adopt | A test must not read the host home. This extends [hermetic-driver-tests.md](../conventions/hermetic-driver-tests.md). |
| Parallel heavy hermetic tests (F3.8) | Adopt | The time of independent slow tests overlaps. |
| A wall budget for each package (F3.7) | Adopt | It shows growth before the 10-minute limit fails CI. |
| Raise the `go test` timeout only | Reject | It hides the growth. A budget with a report comes first. |
| One home for each helper, with a duplicate ratchet (F4) | Adopt | The kits exist. A ratchet stops new copies at the source. |
| Predicate helpers in `pkg/acsassert` (F4.2) | Adopt | 597 copies come from one missing kit. |
| Seams in dependency structs first (F5.4, F5.5) | Adopt | The test stays parallel. A package variable stays the fallback. |
| `testing/synctest` (P6.2) | Defer | The module baseline is Go 1.23 (`go/go.mod`), and CI has a Go 1.23 lane. |
| Learned test selection for gates (P6.5) | Reject | A gate must not miss a red. The full suite is still affordable. |
| Change-scoped selection for lane iteration (P6.6) | Adopt, in shadow | It is fast feedback inside a lane. The full floor stays before each pull request. |
| Shard CI over more runners | Defer | CI wall time is not the limit today. The local floor is the limit. |

## 8. Refinements to adopt

| Id | Refinement | From |
|---|---|---|
| R1 | `test-integration` is the only instrumented runtime run. Every coverage gate reads its `coverage.txt`. | F2.2 to F2.6 |
| R2 | The run writes `coverage.txt` under its final name only when every test passed. | F2.7 |
| R3 | `cover-strict` computes its per-package percents from the profile, through `covchanged.ParseProfile`. | F2.6 |
| R4 | The local floor drops `make test`. `make test` stays as the fast loop of a developer. | F2.1 |
| R5 | `ScanConfigRoot` skips a transcript file whose modification time is before the window start. | F3.4 to F3.6 |
| R6 | A test that builds a real bridge engine pins the Claude config root to a temporary directory. | F3.4 |
| R7 | Independent slow tests call `t.Parallel`. The skills-drift tests are the first set. | F3.8, F3.9 |
| R8 | Each package has a wall budget under `-race`. A report shows each package over its budget. | F3.1, F3.7 |
| R9 | A duplicate ratchet counts equal helper bodies in runtime test files. The count can only go down. | F4.1, F4.3 |
| R10 | `pkg/acsassert` gets the predicate helpers of F4.2. Predicate authors use them. | F4.2 |
| R11 | Test lanes that run at the same time use names that are specific to their file. The merge train builds the package after each merge. | F4.6 |
| R12 | A new seam is a field of a dependency struct, or a parameter. A package variable is the fallback. | F5.4, F5.5 |
| R13 | Lanes can iterate on the importer closure of their change. The full floor runs before each pull request. | P6.6 |

## 9. The timeout investigation

The operator wrote this on 2026-10-12: "You should fix the timeout issue and have investigation done first to learn why timeout happened". This section is that investigation. It used the `go test -json` stream of §3 and three new measurements on `main` at `250fb620b`.

**F9.1** In the run of F3.7, `internal/core` failed at 604 s with "test timed out after 10m0s". Its 1,355 top-level tests that finished all ran serially, for 596 s. 594 tests that call `t.Parallel` paused, and none of them started before the timeout (`go test -json` actions: 1,951 run, 594 pause, 1,355 pass).
*Implication:* Go runs every serial test of a package before it releases the paused parallel tests. The serial sum is the floor of the package wall time, and in `internal/core` that sum alone reached the limit.

**F9.2** The serial tests are serial by omission, not by need. A scan of each test body found:

| Package | Serial tests with no process-state use | Serial tests that set the environment, the directory or the standard streams | Serial tests that swap a package variable |
|---|---|---|---|
| `internal/core` | 1,307 tests, 563 s | 36 tests, 18 s | 11 tests, 15 s |
| `internal/phases/ship` | 593 tests, 567 s | 3 tests, 7 s | 2 tests, 0 s |

Only five helpers in `internal/core` change process state (`chdirTempNonGit`, `demotionRunE4`, `fakeGoOnPath`, `newCycleSourceGitFixture`, `stubGoTool`), and the tests swap 16 package variables. The scan reads test bodies, so a helper can hide a state change; the five helpers above are the ones that it found.
*Implication:* about 97% of the serial time of each package can run in parallel, after a check of the helpers that each test calls.

**F9.3** The slow tests spend their time in `git` processes. A logging `git` on `PATH` counted these calls for the 15 slowest serial tests of `internal/core`, each run alone:

| Test | `git` calls | Alone |
|---|---|---|
| `TestUnwindShipCommit_DeclinesWithoutMovingHEAD` | 500 | 14.3 s |
| `TestRouteRebasedExplanation_ACarryThatCannotBeProvenReturnsToAudit` | 464 | 17.5 s |
| `TestShipRecovery_ContentionBudgetScalesWithFleetWidth` | 318 | 5.9 s |
| `TestVerdictCacheCollisionRegression` | 277 | 7.5 s |
| `TestOrchestrator_AuditLeakRecover` | 214 | 5.2 s |

A `git` call costs about 25 to 35 ms on this host, and more under load.
*Implication:* the cost of a slow test is mostly process starts, so it grows with the load of the host. This is why the same package took 352 s on a quiet host and more than 600 s with four lane floors (F3.7).

**F9.4** Two sources make the calls. Test fixtures build repositories with many commits and checkouts (for example, 42 commits and 36 checkouts in `TestUnwindShipCommit_DeclinesWithoutMovingHEAD`). Production code repeats one query: `git status --porcelain -uall` ran 234 times in `TestShipRecovery_ContentionBudgetScalesWithFleetWidth`, about 8 times in each recovery try (`git_porcelain.go:40`, `leak_recovery.go:146`, `continuation_stamp.go:31`).
*Implication:* a fixture template cuts the setup cost of every test. A cache of the status result inside one recovery try cuts a cost that the live loop also pays.

**F9.5** Eight copies of one `git`-bound test (`TestRouteRebasedExplanation_ACarryThatCannotBeProvenReturnsToAudit`) ran together in 15.1 s; one copy alone took 8.1 s. The host has 14 cores, and its load average was 13 to 17 during the run.
*Implication:* parallel runs of these tests give about 4.3 times the throughput even on a loaded host. At that rate, the 563 s of serial `internal/core` time becomes about 130 s.

**F9.6** The history explains part of the serial state of `internal/phases/ship`. Its real-git tests were serial for three reasons. `t.Setenv` panics in a parallel test, and package-variable seams race. Also, a darwin EBADF flake in file-descriptor teardown showed under `-race -count=3 -shuffle=on` ([go/docs/testing.md](../../go/docs/testing.md), "Why ship's fast tier was hard to parallelize"). `tempRepoDir` now contains the EBADF case with a best-effort cleanup.
*Implication:* the move to parallel tests in `internal/phases/ship` needs a stress gate (`-race -count=3 -shuffle=on`) for each file, not a blind change.

**Root cause.** A longer timeout does not fix the defect. The defect is that about 1,900 tests in the two largest packages run one after another, each one starting hundreds of `git` processes. The serial sum grows with the suite and with the load of the host, and it reached the 10-minute limit.

| Id | Refinement | From |
|---|---|---|
| R14 | Tests in `internal/core` and `internal/phases/ship` call `t.Parallel` unless they change process state. A structure test keeps the rule. | F9.1, F9.2 |
| R15 | Each file moves to parallel only after `-race -count=3 -shuffle=on` passes for its package. | F9.6 |
| R16 | A repository template, built one time for each test binary, replaces the repeated setup of the same fixture. | F9.3, F9.4 |
| R17 | One recovery try reads `git status --porcelain -uall` one time and passes the result on. | F9.4 |
| R18 | The 20-minute timeout and the package budget (T4) come after R14, as a guard and an alarm, not as the fix. | F9.1 to F9.5 |
