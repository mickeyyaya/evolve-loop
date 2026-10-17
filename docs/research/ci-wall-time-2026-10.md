# CI wall time: where the 21–25 minutes go, measured from the test up (research, 2026-10)

> The plan that uses this research: [ci-wall-time-2026-10](../plans/ci-wall-time-2026-10.md). The design: [ci-telemetry-and-critical-path](../architecture/ci-telemetry-and-critical-path.md). The decision record: [ADR-0135](../architecture/adr/0135-ci-measures-itself-before-it-changes-its-shape.md). Earlier work on the same suite: [test-system-efficiency-2026-10](test-system-efficiency-2026-10.md) and its [plan](../plans/test-system-efficiency-2026-10.md).

Date: 2026-10-17. Method:

- job and step timings for six `required CI` runs on main (`gh run view --json jobs`);
- per-package times from the full log of run 38974705695;
- a local `go test -json` profile of the five slowest packages, and a race-cost run;
- a code read of the slowest tests, and a web survey;
- for F3.20 to F3.35: the artifacts of the C0b run 39031338517, four main runs and the PR #935 run. Section 3.1 gives the method.

Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

- [1. The request](#1-the-request)
- [2. The measurement model](#2-the-measurement-model)
- [3. Local evidence](#3-local-evidence)
  - [3.1 The first instrumented run (C0b)](#31-the-first-instrumented-run-c0b)
- [4. Current practice](#4-current-practice)
- [5. Adopt or reject](#5-adopt-or-reject)
- [6. Refinements to adopt](#6-refinements-to-adopt)

## 1. The request

On 2026-10-17 the operator asked: "Ultrathink to improve the efficiency of ci time, it takes way too long to complete, deep dive and research to investigate". Then: "Think from the smallest unit to improve the telemetry and how we measure the time and find the critical path and identify the bottleneck". Then: "Please ensure that everything is documented in detail, propose at least three different approaches, and identify the winning solution while outlining the advantages of the alternatives."

## 2. The measurement model

The CI time is a tree of units. Each level has its own rule for how time adds up. The critical path is the chain of units that sets the total.

| Level | Unit | How its time adds up |
|---|---|---|
| L1 | One test | Its own elapsed time. A test that calls `t.Parallel()` waits, then runs beside others. A test that does not call it runs alone. |
| L2 | One package | Compile and link, then the serial tests one after another, then the parallel phase. The floor is (Σ serial tests) + (longest parallel test). |
| L3 | The test step | `go test` runs up to `-p` packages at a time (the CPU count). The floor is the longest package. |
| L4 | One job | Its steps run one after another: checkout, setup, vet, build, test, e2e, gates. |
| L5 | The workflow | The jobs run at the same time. The total is the longest job. |

*Worked example (synthesis from F3.1 to F3.5):* the workflow takes 24 min because the macOS job takes 24 min (L5). Its test step takes 17.5 min (L4). Inside it, `cmd/evolve` takes 348 s (L3). Inside `cmd/evolve`, 841 serial tests add up to 228 s, which no CPU count can shorten (L2).

## 3. Local evidence

**F3.1** The workflow wall time is the macOS job. Five runs on 2026-10-16 count here; a sixth was still running. In them, macOS 1.23 took 20.7–24.6 min, ubuntu 1.23 took 10.5–12.5 min and ubuntu 1.27 took 9.3–13.3 min. Every other job took 2 min or less (`gh run view <id> --json jobs`, runs 38974705695, 38970817211 and three more).
*Implication:* a change that does not shorten the macOS job does not shorten CI.

**F3.2** In each Go job, the test step is most of the time: 1,024–1,058 s on macOS and 596–616 s on ubuntu. The e2e step adds 168–172 s on macOS and 56–60 s on ubuntu. Checkout, setup, vet and build together take under 2.5 min (step timings of the same runs).
*Implication:* the work to cut is inside `make test-integration` and `make test-e2e`, not in setup.

**F3.3** All three Go jobs run the same full suite with `-race -cover -tags integration` (`go/Makefile:112`, `.github/workflows/go.yml` "test (race + cover, incl. integration tier)"). Each also runs vet, gofmt, e2e and all the coverage gates.
*Implication:* the macOS and Go 1.27 jobs repeat work that ubuntu 1.23 already proves. The exception is coverage of files that compile only on darwin. Ubuntu never compiles them, so they never appear in its `coverage.txt`.

**F3.4** The per-package times in run 38974705695 add up to 1,571 s on ubuntu and 2,539 s on macOS, for 303 packages. The step walls are 600 s and 1,040 s, so each runner overlaps about 2.5 packages.
Ubuntu has 4 vCPU and macOS has 3: GitHub's standard sizes, not measured here.

The slowest packages on ubuntu are `phases/audit` 198 s, `bridge` 194 s, `cmd/evolve` 143 s and `phases/ship` 124 s. On macOS they are `cmd/evolve` 348 s, `phases/audit` 311 s, `bridge` 304 s and `core` 255 s.
*Implication:* at L3 the floor is about 200 s on ubuntu and 350 s on macOS, set by single packages. More runners or more CPUs cannot go below the longest package.

**F3.5** A local `go test -json` profile of the five slowest packages (`-race -cover -tags integration`, 14 CPUs, 2026-10-17) splits each package into its serial and parallel phases:

| Package | Wall | Serial tests (Σ) | Longest parallel test | What sets the floor |
|---|--:|---|--:|---|
| `cmd/evolve` | 255 s | 841 tests, 228 s | 23.6 s | the serial chain |
| `core` | 182 s | 158 tests, 76 s | 60.4 s (`TestCycleScenarios`) | both |
| `bridge` | 140 s | 112 tests, 65 s (42 s is `TestPaneParity_ReportsTheCycle1853TriggerWhenThePtyRulesAreRemoved`) | 48.4 s (`TestRunTmuxREPL_TransientDwell_BusyPaneIsNeverPreempted`) | both |
| `phases/audit` | 113 s | 38 tests, 3 s | 106.0 s | one parallel cluster |
| `phases/ship` | 99 s | 30 tests, 6.5 s | 38.1 s | the parallel phase |

Compile and link of the five packages takes 11 s with a warm cache. The serial and parallel split comes from the `pause` events of the `-json` stream: a test that never emits `pause` is serial. A short script made the split. `cmd/testlatency` reports only the sum over all tests (`SerialSum`) and the slowest test (`go/cmd/testlatency/report.go`).
*Implication:* the bottlenecks have three different shapes. Each one needs its own fix (F3.6 to F3.8).

**F3.6** Nine `TestSkillsDriftGate_*` tests in `phases/audit` take 79–106 s each, about 750 s in total locally (9 tests × 79–106 s). Each one calls `skillsDriftCheckDefault`, which runs `go` with `core.WorktreeEvolveInvocation(root, "skills", "check")` in a copied module (`go/internal/phases/audit/skillsdrift.go:42-47`). That compiles and links the whole `evolve` binary once per test, nine at a time.
*Implication:* one cluster takes a large share of the suite: about 750 s locally, against 1,571 package-seconds for the whole ubuntu suite on CI. Its share on CI is not measured, and it slows every package beside it.

**F3.7** `cmd/evolve` has 841 top-level tests that never call `t.Parallel()`. Their sum, 228 s, is 89% of the package wall. The slowest are `TestWorktreeDevCleanup_EveryGitFaultFailsTheTreeAndKeepsIt` (8.5 s), `TestLegacyAdvisorDispatch_MatchesTheGolden` (7.8 s) and `TestRunLoop_ResetErrorPathsKeepBasePrefixes` (7.4 s).
*Implication:* a serial chain runs in one process. Splitting the tests across processes (`go test -run` lists in separate jobs) parallelizes it without a code change.

**F3.8** The `TransientDwell` and `PaneParity` tests in `bridge` take 33–48 s each, and they drive a real tmux pane through a dwell timer. F3.25 corrects this: the dwell tests run a simulated tick loop, so an injected clock cannot shorten them.
*Implication (synthesis):* their time is mostly real waiting, which an injected clock removes. This is the same pattern that the earlier campaign fixed in other packages.

**F3.9** Race and coverage cost little here. `go test -count=1 -tags integration ./internal/phases/ship ./internal/bridge` took 74 s with `-race -cover` and 60 s without them (local, 2026-10-17).
*Implication:* for these two packages, removing `-race` saves about 20%. The cost for the whole suite is not measured. The macOS lever is scope (F3.10), not flags.

**F3.10** 16 packages hold darwin-specific code (a scan of `go/` without `go/acs`, 2026-10-17). It counts `_darwin.go` files, `//go:build` lines that name `darwin` without `!`, `runtime.GOOS == "darwin"` branches, and `sandbox-exec` in non-test files.

- Darwin files or tags: `cmd/evolve`, `bridge`, `events/reader`, `events/wake`, `looppreflight`, `phases/ship`, `proctree`, `reportdoc`.
- `GOOS == "darwin"` branches: `adapters/sandbox`, `capability`, `testhost`.
- `sandbox-exec` only: `bedrock`, `cycleclassify`, `flagregistry`, `phasestream`, `preflight`. Some of these name the binary only in a string. The set keeps them, so that it errs on the safe side.

*Implication:* the macOS job adds evidence only for these packages and the e2e tier. The other 287 packages give the same result on both systems.

**F3.11** The CI already writes the finest-grained data, but it keeps none of it. The test step sets `TEST_JSON: test-integration.json` (`.github/workflows/go.yml`), but the artifact step uploads only `coverage.txt` and `apicover-report.txt`. The "package wall budget" step writes a summary for one run and keeps no history.
*Implication:* no one can compare per-test times across runs, find a regression, or balance shards from history. Telemetry is the first gap.

**F3.12** The earlier campaign moved several packages to parallel-by-default and shared fixtures ([test-system-efficiency-2026-10](test-system-efficiency-2026-10.md)). Locally, `cmd/evolve` went from 686 to 211 s and `core` from 215 to 142 s. `bridge` went from 182 to 56 s and `audit` from 185 to 113 s. The CI numbers in F3.4 are higher, because CI runners have 3–4 CPUs, not 14.
*Implication:* parallel-by-default did not reach the remaining poles: tests that must stay serial, one heavy cluster, and real waits.

**F3.13** Time alone hides where a package spends its time. A `go test -exec` wrapper read the OS resource accounting (rusage) of each test binary and its waited-for descendants. Local run on a 14-CPU laptop, `-tags integration`, 2026-10-17:

| Package | Wall | User CPU | System CPU | Peak RSS | Involuntary context switches |
|---|--:|--:|--:|--:|--:|
| `phases/audit` | 106 s | 163 s | 417 s | 325 MB | 2.1 M |
| `phases/ship` | 87 s | 88 s | 293 s | 97 MB | 1.35 M |
| `gitexec` | 2.7 s | 0.9 s | 1.2 s | 13 MB | 13.5 k |

The prototype is a 40-line Go program. `go test -exec <wrapper>` starts one wrapper process per test binary, so the numbers are per package.
*Implication:* the two slowest packages spend 2.5–3.3 times more CPU in the kernel than in user code. The cost is process creation and filesystem calls (`git` and `go` subprocesses).

The high count of involuntary context switches suggests scheduler contention, which package rusage cannot prove (F3.15). It is not memory. A fix that cuts subprocesses helps more than a faster CPU.

**F3.14** macOS reports 0 for block reads and writes (`inblock`, `oublock`) in this run, while Linux fills them. macOS reports peak RSS in bytes and Linux in kilobytes.
*Implication:* the wrapper must scale peak RSS per OS, and block I/O is trustworthy only on the ubuntu jobs.

**F3.15** Package rusage cannot see contention between packages, or a runner that waits on disk. Only a sampler of the whole runner can show utilization and saturation (the USE view). It reads CPU busy, run-queue length, memory, swap and disk wait.
*Implication:* the telemetry needs two layers: per-package resource accounting, and one runner sampler per job.

### 3.1 The first instrumented run (C0b)

On 2026-10-17 the operator asked: "Continue investigating the performance, latency, resource usage for ci pipeline and test cases". An earlier request, "Identify which test starves the CPU and improve the test for parallel running", gave PR #935 (C3 and C5b). F3.20 to F3.35 answer the new request. PR #935 adds F3.16 to F3.19, R10, D14 and C5b; this section does not reuse those ids.

**Method.** All data comes from CI through the `gh` CLI, so it costs the dev host nothing:

| Source | Runs | What it gives |
|---|---|---|
| `gh run download 39031338517` | the C0b run (branch `feat/ci-c0b-rusage`, green) | per job: `test-rusage.ndjson` (301 integration and 9 e2e records), `runner-samples.ndjson`, `test-integration.json`, `test-e2e.json` |
| `gh run view <id> --json jobs` | 39031338517 and the five main runs 39030259275, 39029070762, 39027330315, 39026408729, 39024516399 | job and step times |
| `gh run download <id> -n coverage-<os>-go1.23` | four green main runs of the five | the `-json` streams, for the variance across runs |
| `gh run download 39033417060` | the PR #935 run (C3 and C5b) | the ubuntu streams after the fix |
| `evolve ci telemetry 39031338517 --json` and `--history 5` | built from branch `feat/ci-c1-critical-path` | a test of the C1 report (F3.34) |

The rusage file holds two records for each `cmd/*` package: the integration step writes the first, and the e2e step appends the second. The analysis takes the first record of a package as the integration record. Short scripts in the scratch directory made the tables. "u23" is the ubuntu Go 1.23 job, "u27" is the ubuntu Go 1.27 job, and "mac" is the macOS Go 1.23 job.

**F3.20** The ubuntu runner speed has two modes. In four of the five main runs, the u23 test step took 604–615 s. In main run 39024516399 it took 477 s, and in the C0b run 432 s. The sum of the package walls is 1,607–1,617 s in three main runs, 1,312 s in 39024516399 and 1,160 s in the C0b run.
*Implication:* the C0b u23 numbers come from a fast runner, about 0.7× of the main median. A before-and-after claim must compare runs of the same mode, or use the median of five runs or more.

**F3.21** The critical path of the C0b run, from L5 to L1:

| Level | macOS 1.23 | ubuntu 1.23 | ubuntu 1.27 |
|---|---|---|---|
| L5 job wall | 1,523 s (the workflow path; the run took 1,600 s) | 553 s | 776 s |
| L4 test step | 1,168 s | 432 s | 606 s |
| L4 e2e step | 176 s | 42 s | 59 s |
| L3 longest package | `core` 362 s | `phases/audit` 135 s | `phases/audit` 209 s |
| L3 Σ package wall / step wall | 2,755 / 1,168 = 2.4 | 1,161 / 432 = 2.7 | 1,642 / 606 = 2.7 |
| L2 floor of that package | serial 54.7 s + parallel 37.3 s = 92 s | longest parallel test 84.5 s | — |
| L1 test | `TestSkillsDriftGate_CommittedGeneratorChangeIsGradedByTheWorktreeWithOrWithoutTheHost` 132 s | `TestSkillsDriftGate_ImportedPackageChangeIsGradedByTheWorktreeGenerator` 84.5 s | — |

On macOS, the longest package is 31% of the step, and its wall is 3.9× its floor.
*Implication (synthesis):* no single pole sets the step. The step is throughput-bound: the sum of work over the slots. Section F3.23 shows the limit on each system.

**F3.22** macOS is the longest job because each unit of work costs more in the kernel. Over the 301 integration binaries, macOS used 1,282 s of user CPU and 1,006 s of system CPU. ubuntu 1.23 used 861 s and 160 s. So system CPU is 6.3× on macOS, and user CPU is 1.5×. macOS also has 3 vCPU and `-p 3`; ubuntu has 4 and `-p 4`. The top 20 packages by macOS wall:

| Package | mac wall | mac CPU | mac sys | u23 wall | u23 CPU | u23 sys | wall ratio | sys ratio |
|---|--:|--:|--:|--:|--:|--:|--:|--:|
| `internal/core` | 362 | 482 | 310 | 33 | 62 | 24 | 10.9 | 12.7 |
| `cmd/evolve` | 348 | 313 | 150 | 94 | 92 | 18 | 3.7 | 8.2 |
| `internal/phases/audit` | 323 | 398 | 129 | 135 | 344 | 34 | 2.4 | 3.8 |
| `internal/bridge` | 315 | 276 | 54 | 106 | 169 | 11 | 3.0 | 4.7 |
| `internal/phases/ship` | 204 | 261 | 143 | 74 | 78 | 22 | 2.8 | 6.5 |
| `internal/explanationdocs` | 82 | 55 | 31 | 7 | 6 | 3 | 11.2 | 10.6 |
| `internal/practicewatch` | 78 | 11 | 1 | 34 | 10 | 1 | 2.3 | 1.0 |
| `internal/ciparity` | 70 | 86 | 34 | 36 | 54 | 9 | 1.9 | 3.6 |
| `internal/gc` | 65 | 44 | 24 | 6 | 5 | 3 | 10.1 | 9.2 |
| `internal/deliverable` | 59 | 18 | 10 | 40 | 3 | 1 | 1.5 | 8.2 |
| `internal/phases/runner` | 32 | 7 | 2 | 29 | 4 | 0.4 | 1.1 | 5.5 |
| `internal/repocontract` | 31 | 25 | 2 | 23 | 20 | 1 | 1.3 | 2.2 |
| `internal/releasepipeline` | 24 | 30 | 7 | 16 | 32 | 3 | 1.5 | 2.5 |
| `internal/changedpkgs` | 23 | 16 | 7 | 20 | 12 | 2 | 1.2 | 3.1 |
| `internal/guards` | 23 | 13 | 2 | 7 | 6 | 0.3 | 3.2 | 7.1 |
| `internal/inboxstamps` | 22 | 13 | 8 | 3 | 2 | 1 | 7.2 | 8.0 |
| `internal/phases/triage` | 22 | 10 | 6 | 8 | 2 | 1 | 2.7 | 6.7 |
| `internal/adapters/ledger` | 18 | 15 | 4 | 12 | 9 | 1 | 1.5 | 3.3 |
| `internal/rawgitratchet` | 15 | 9 | 1 | 17 | 8 | 0.3 | 0.9 | 3.1 |
| `internal/phasecoherence` | 13 | 5 | 1 | 4 | 4 | 1 | 3.1 | 1.2 |

All times are seconds.

The packages with the largest wall ratio (`core`, `explanationdocs`, `gc`, `inboxstamps`) are also the packages with the largest system ratio. `internal/routingtest`'s `RunAll` gives each `core` scenario a copy of one template repository. Then the `RunCycle` path starts about 10 to 15 `git` processes per scenario: `worktree add`, `rev-parse`, `status`, `log` and `worktree remove` (`go/internal/routingtest/engine.go:222`, `go/internal/core/worktree.go:43-59`, a code read).
*Implication:* the macOS cost is process creation and file-system calls, not the test code. A package that starts many `git` processes is 4–13× slower on macOS. A package that only waits (`practicewatch`, `phases/runner`) is about 1–2× slower.

**F3.23** The ubuntu runner is CPU-saturated during the test step. The macOS step fills its `-p` slots, but no sampler measures its CPU.

| Index over the test step | u23 (432 s) | u27 (606 s) | mac (1,168 s) |
|---|--:|--:|--:|
| CPU busy, mean / P50 / P95 | 80 / 97 / 100% | 92 / 100 / 100% | not recorded |
| Seconds with CPU busy ≥ 95% | 57% | 78% | not recorded |
| Seconds with CPU busy < 50% | 17% | 6% | not recorded |
| Run queue, mean / P95 (4 vCPU) | 12 / 27 | 14 / 29 | not recorded |
| Memory used, max | 3.0 GB | 3.1 GB | 4.3 GB |
| Swap used, max | 0 | 0 | 0 |
| Disk busy, P95 | 56% | 16% | not recorded |
| Seconds with 4 packages running | 139 of 409 | 216 of 592 | — |
| Seconds with 3 packages running (the mac `-p`) | 134 | 152 | 644 of 1,150 |

The u23 busy profile per minute was 62, 93, 86, 62, 92, 99, 76 and 33%. The low minutes are the start (compile) and the tail (the last short packages).

The macOS sampler writes only memory and swap: on branch `feat/ci-c0b-rusage`, `go/cmd/hostsampler/source_darwin_test.go:16` asserts that the darwin source has no CPU and no run queue. The design (section 6 of the architecture doc) names `host_statistics` as the macOS CPU source, so the built sampler differs from its spec.
*Implication:* on ubuntu, a larger `-p` cannot help, because the run queue is already 3× the CPU count. Fewer CPU-seconds, or more jobs, can help. On macOS, the evidence cannot decide between CPU and slots until the sampler records CPU.

**F3.24** CPU hogs: the top 20 packages by user plus system CPU on u23, with the same packages on macOS.

| Package | u23 CPU s | share | CPU/wall | sys share | mac CPU s | mac share | mac sys share |
|---|--:|--:|--:|--:|--:|--:|--:|
| `internal/phases/audit` | 344 | 33.7% | 2.5 | 10% | 398 | 17.4% | 32% |
| `internal/bridge` | 169 | 16.5% | 1.6 | 7% | 276 | 12.1% | 20% |
| `cmd/evolve` | 92 | 9.0% | 1.0 | 20% | 313 | 13.7% | 48% |
| `internal/phases/ship` | 78 | 7.6% | 1.1 | 28% | 261 | 11.4% | 55% |
| `internal/core` | 62 | 6.0% | 1.8 | 40% | 482 | 21.1% | 64% |
| `internal/ciparity` | 54 | 5.3% | 1.5 | 17% | 86 | 3.8% | 40% |
| `internal/releasepipeline` | 32 | 3.1% | 2.0 | 9% | 30 | 1.3% | 25% |
| `internal/repocontract` | 20 | 1.9% | 0.8 | 4% | 25 | 1.1% | 6% |
| `internal/verifyeval` | 15 | 1.5% | 2.0 | 9% | 11 | 0.5% | 18% |
| `internal/changedpkgs` | 12 | 1.2% | 0.6 | 18% | 16 | 0.7% | 41% |
| `internal/practicewatch` | 10 | 0.9% | 0.3 | 9% | 11 | 0.5% | 8% |
| `internal/adapters/ledger` | 9 | 0.8% | 0.7 | 15% | 15 | 0.6% | 28% |
| `internal/rawgitratchet` | 8 | 0.8% | 0.5 | 3% | 9 | 0.4% | 8% |
| `internal/bridge/panestream` | 8 | 0.8% | 0.8 | 6% | 8 | 0.4% | 7% |
| `internal/guards` | 6 | 0.6% | 0.9 | 6% | 13 | 0.6% | 19% |
| `internal/explanationdocs` | 6 | 0.5% | 0.8 | 51% | 55 | 2.4% | 55% |
| `test/e2e` | 5 | 0.5% | 1.6 | 6% | 8 | 0.4% | 16% |
| `internal/gc` | 5 | 0.5% | 0.8 | 52% | 44 | 1.9% | 55% |
| `internal/testmainexit` | 4 | 0.4% | 0.8 | 2% | 6 | 0.2% | 4% |
| `internal/evalqualitycheck` | 4 | 0.4% | 0.6 | 3% | 5 | 0.2% | 10% |

The total is 1,021 CPU-s on u23 and 2,288 CPU-s on macOS. The top four packages use 67% of the u23 CPU. F3.16 of PR #935 gives the same order for u27 run 39027224638. The high-`sys` packages start these processes (a code read):
- `phases/ship`: `TestScanShadow_ChangesNoVerdict` runs 2 trees × 6 gate runs, and each is a real `go test` over about 16 packages, plus `git ls-files` and `go env -json` (`go/internal/phases/ship/scanshadow_test.go:197`). `TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests` runs a real `go test` over the 14-package pack, because its first subtest scopes the seam swap to itself (`go/internal/phases/ship/repocontract.go:201`).
- `ciparity`: `TestMakeCoverStrict_*` and `TestMakeApicover_*` run real `make test-integration`, `make cover-strict` and `go run ./cmd/evolve` (`go/internal/ciparity/makefile_integration_test.go:214`, `:397`, `:428`).
- `core`, `gc`, `explanationdocs`, `inboxstamps`: `git` processes in temporary repositories (F3.22).

*Implication:* C3 (PR #935) targets the largest hog. After `phases/audit`, the next CPU is `bridge` (simulated tick loops, F3.25) and the real `go test` and `make` children of `phases/ship` and `ciparity`.

**F3.25** Wall-time poles with low CPU. Per-test CPU is not recorded, so this table joins three signals. The first is the package CPU/wall. The second is the macOS-to-ubuntu ratio: a test that waits takes the same time on both systems. The third is a code read of each test.

| Test | u23 s | mac s | What the time is | Source |
|---|--:|--:|---|---|
| `practicewatch` `TestWatch_AnOversizedBodyIsRecordedWithATruncationNote` | 20.7 | 62.5 | 2 fetches of an 8 MiB one-line body from an `httptest` server; a blank-run regex then scans 8 MiB under `-race` | `go/internal/practicewatch/hardening_test.go:311`, `normalize.go:41-49` |
| `deliverable`: 185 serial tests, 35.6 s, 3 s CPU | 35.6 | 54.6 | each read of a missing or empty deliverable sleeps up to 500 ms (25 polls of 20 ms) | `go/internal/deliverable/deliverable.go:120-141` |
| `phases/runner` `TestRun_MandatoryPhase_ArtifactTimeout_StillFails` and `TestRun_OptionalPhase_ArtifactTimeout_DegradesToWarn` | 8.2 each | 8.2 each | 16 verify calls (1 + `SettleRetries` 15) × the 500 ms grace sleep above | `go/internal/phases/runner/verdict/verdict.go:33` |
| `rawgitratchet` `TestRatchet_NoNewRawGitFixtures` | 15.7 | 13.9 | parses about 4,441 `_test.go` files (21 MB) of the real tree | `go/internal/rawgitratchet/rawgitratchet.go:25-145` |
| `cmd/evolve` `TestRatchet_CheckPassesOnTheRealModuleAndRejectsOtherVerbs` | 30.5 | 27.3 | the same tree parse, plus a parse of about 1,610 production files | `go/cmd/evolve/cmd_ratchet_test.go:14` |
| `repocontract` `TestNoTestWritesIntoTheRealTree`, `TestPackages_HoldEveryTestThatReadsTheWholeTree` | 11.2, 10.9 | 18.6, 11.2 | each parses the same 4,441 test files again | `go/internal/repocontract/realtree_writes_test.go:95`, `packages_test.go:75` |
| `changedpkgs` `TestImporterClosure_*` (36 serial tests, 18.6 s) | 3.4 max | 4.0 max | 10 full-module `go list -e ./...` processes | `go/internal/changedpkgs/importerclosure.go:98` |
| `bridge` `TestRunTmuxREPL_TransientDwell_*` (3 tests) | 9.6–12.1 | 23.6–43.8 | not a wait: a simulated loop of about 150 ticks with a fake sleep; each tick runs the regexes over a 4 KB pane | `go/internal/bridge/driver_tmux_wait.go:54` |
| `bridge` `TestRunTmuxREPL_ACaptureFailureOnALiveSessionDoesNotEndTheDispatch`, `_OneTickWithoutTheSessionIsNotYetAPaneLoss`, `_AHealthyCaptureNeverQueriesTheSession` | 19.8–26.8 | 12.9–24.0 | the same loop, 600 to 1,200 ticks | `go/internal/bridge/driver_tmux_wait_paneloss_test.go:59-90` |

On u23, twelve packages have CPU/wall < 0.3 and more than 5 s of wall. They are `deliverable` (0.08), `practicewatch` (0.28), `phases/runner` (0.13), `phases/triage`, `swarm`, `phases/tdd`, `phases/intent`, `looppreflight`, `cli/phasecmd`, `sysexec`, `phaseobserver` and `pkg/phaseproto`.
*Implication:* the plan's names for these tests were partly wrong. The `TransientDwell` tests do not wait: they burn CPU in a simulated loop, so an injected clock (C5) cannot help them; fewer ticks can. The real waits are the 500 ms deliverable grace and its 16× multiple in `phases/runner`. On a saturated runner, CPU/wall < 1 can also mean CPU starvation, so a low ratio alone does not prove a sleep.

**F3.26** Every test binary under `-race` sleeps 1 s before it exits. The race detector option `atexit_sleep_ms` has a default of 1,000 ms (P4.9). In the C0b run, no integration binary took less than 1.008 s on u23, 1.010 s on u27 or 1.025 s on macOS. 189 of the 301 u23 binaries took 1.0–1.2 s, with as little as 0.002 s of CPU (for example `cmd/commentaudit`, 1.016 s wall).

The same packages in the e2e step, without `-race`, mostly took 0.002–0.05 s. The exceptions are `cmd/evolve` (33 s), `test/e2e` (6.3 s) and `hostsampler` (3.1 s), which run real work there.
*Implication:* each Go job pays 301 slot-seconds of sleep. With 3 slots on macOS, that is up to about 100 s of the test step. With 4 slots on ubuntu it is up to about 75 s, but the CPU is saturated, so less of it is wall. The loop's build floor and each local `go test -race ./...` pay the same sleep.

**F3.27** The e2e step runs the whole `cmd/evolve` suite a second time. `make test-e2e` runs `go test -tags "e2e evolve_test_phases" ./cmd/... ./test/e2e/...` (`go/Makefile:128-129`). That runs 2,228 `cmd/evolve` tests, but only 28 top-level tests sit in files with an `e2e` build tag.

The tests that the integration step already ran used 188.9 s of test time on macOS and 34.5 s on u23. The e2e-only tests used 58.2 s and 24.6 s. The step also runs the other `cmd/*` packages again.
*Implication:* about three quarters of the macOS e2e step (176 s) repeats tests that already passed under `-race`. The repeat adds one signal: the same tests pass without `-race` and with the `evolve_test_phases` tag.

**F3.28** Memory is not a risk on the runners, but the largest packages hold the whole tree in memory.

| Package | u23 peak RSS | mac peak RSS | u27 peak RSS | What holds it |
|---|--:|--:|--:|---|
| `internal/repocontract` | 1,222 MB | 1,382 MB | 1,269 MB | all ASTs of about 4,441 test files at once, under one `FileSet` (`byPackage` and `byDir`) |
| `cmd/evolve` | 884 MB | 901 MB | 778 MB | the tree scans of F3.25 and a 2,332-test binary |
| `internal/phasecoherence` | 604 MB | 722 MB | 770 MB | not read |
| `internal/core` | 477 MB | 572 MB | 520 MB | not read |
| `internal/phases/ship` | 343 MB | 314 MB | — | not read |

The runner memory peaked at 3.0 GB (u23), 3.1 GB (u27) and 4.3 GB (macOS), with no swap. GitHub's standard runners have 16 GB (ubuntu) and 7 GB (macOS arm64), which this doc does not measure.

A dev host with 14 CPUs runs `-p 14`. If the five largest packages run together, they need about 3.7 GB (synthesis: 1.2 + 0.9 + 0.6 + 0.5 + 0.5). The live wave runs 4 lanes, so 4 floors at once can need about 15 GB at the worst moment.
*Implication:* there is no OOM risk on CI. The dev-host risk is real only when several lanes run the full suite together. A streamed parse in `repocontract` (one package at a time) removes the largest peak.

**F3.29** Serial chains: the top 14 packages by serial sum on macOS. A test is serial when it never emits `pause`.

| Package | serial / all tests | mac serial Σ | mac wall | u23 serial Σ | u23 wall |
|---|--:|--:|--:|--:|--:|
| `cmd/evolve` | 842 / 2,332 | 238.2 s | 348 s | 49.6 s | 94 s |
| `internal/explanationdocs` | 170 / 175 | 81.0 s | 82 s | 6.3 s | 7 s |
| `internal/practicewatch` | 69 / 69 | 76.5 s | 78 s | 32.9 s | 34 s |
| `internal/gc` | 210 / 251 | 63.9 s | 65 s | 5.4 s | 6 s |
| `internal/core` | 163 / 2,174 | 54.7 s | 362 s | 9.0 s | 33 s |
| `internal/deliverable` | 185 / 220 | 54.6 s | 59 s | 35.6 s | 40 s |
| `internal/bridge` | 112 / 1,260 | 43.0 s | 315 s | 3.5 s | 106 s |
| `internal/repocontract` | 19 / 19 | 30.4 s | 31 s | 22.2 s | 23 s |
| `internal/phases/runner` | 216 / 229 | 29.7 s | 32 s | 27.2 s | 29 s |
| `internal/phases/ship` | 30 / 684 | 27.8 s | 204 s | 19.5 s | 74 s |
| `internal/releasepipeline` | 137 / 137 | 22.5 s | 24 s | 14.7 s | 16 s |
| `internal/changedpkgs` | 36 / 41 | 22.3 s | 23 s | 18.6 s | 20 s |
| `internal/guards` | 145 / 145 | 21.5 s | 23 s | 5.9 s | 7 s |
| `internal/inboxstamps` | 31 / 31 | 20.7 s | 22 s | 2.0 s | 3 s |

The total serial sum is 1,111 s on macOS and 431 s on u23. The parallel guard of `cmd/evolve` keeps its allow-list in `go/cmd/evolve/parallel_serial_tests_test.go:3`: 671 entries, each with a reason. The guard is `go/cmd/evolve/parallel_default_test.go:19`. Tests that call `t.Setenv` are exempt, so they are not in the list. The cost of the `cmd/evolve` serial tests by reason:

| Reason | Tests | mac Σ | u23 Σ |
|---|--:|--:|--:|
| "swaps seam X" | 312 | 92.9 s | 14.6 s |
| "sets env" (254 "through a helper") | 257 | 78.4 s | 15.6 s |
| not in the list (`t.Setenv`, exempt) | 178 | 59.2 s | 17.8 s |
| other reasons (`os.Pipe`, `runLoop`, `devSweepContext`) | 30 | 4.8 s | 0.9 s |
| `SetHangClassifier`, `SetModelCatalogDirFn`, `runner.DefaultRouter` | 65 | 3.1 s | 0.9 s |

The slowest serial test is `TestLegacyAdvisorDispatch_MatchesTheGolden`: 41.2 s on macOS and 10.3 s on u23. It builds 120 routers over the real profile directory (`go/cmd/evolve/cmd_router_dispatch_golden_test.go:34`).
*Implication:* on macOS, the `cmd/evolve` serial chain (238 s) is 68% of its package wall. Seam swaps and env writes cause 171 s of it. Plan C4 must move the seams and the env into injected values, test by test, or the chain stays.

**F3.30** The steps outside `go test` cost 2–4 min per job, and some work repeats.

| Step | mac | u23 | u27 | Repeated work |
|---|--:|--:|--:|---|
| checkout (`fetch-depth: 0`) | 29–45 s | 21–44 s | 21–27 s | full history in each of 3 jobs |
| setup-go (with cache) | 24–43 s | 9–11 s | 16–20 s | — |
| install tmux (`brew`) | 12–16 s | — | — | each run |
| vet + fmt | 37–59 s | 35–49 s | 38–51 s | the same `go vet ./...` and `gofmt -d -s .` in 3 jobs |
| build (`make build`) | 6–15 s | 2–4 s | 2–4 s | — |
| test (race + cover) | 815–1,208 s | 432–615 s | 428–619 s | — |
| cover-changed, cover-strict, apicover × 2, function warnings | 1–4 s each | 0–1 s each | 0–1 s each | `go tool cover -func` reads the 4.9 MB profile 4 times; `cmd/apicover` is built twice; `cover-strict` and `cover-changed` each `go run ./cmd/evolve` |
| e2e tier | 118–176 s | 42–59 s | 45–59 s | F3.27 |

The ranges cover the C0b run and the five main runs. The jobs off the path take 26–76 s each: `acs-durable` 76 s, `landing` 60 s, `STE lint` 47 s, `govulncheck` 43 s.
*Implication:* the largest repeat is the e2e step (F3.27). Next come vet and gofmt in three jobs: up to about 50 s of each of two jobs. `GOOS=darwin go vet` on ubuntu can type-check the darwin files, so macOS does not need its own vet (synthesis).

**F3.31** The three tests of inbox item `three-tests-flake-under-full-suite-load` are short and sensitive to load, not heavy:

| Test | u23 elapsed in 5 runs | mac elapsed in 5 runs | Timing in the code |
|---|---|---|---|
| `bridge` `TestChannelE2E_RealFixtures_ClaudeSpan` | 0.32, 0.79, 0.51, 1.63, 0.50 s | 0.97, 1.40, 0.35, 0.46, 0.95 s | a fake `Sleep` that switches the pane on the breadcrumbs of a producer goroutine with a 1 ms poll (`go/internal/bridge/channel_e2e_test.go:43-60`) |
| `bridge/channel` `TestChannel_EndToEnd` | 0.00–0.01 s | 0.00–0.01 s | a 1 ms producer poll and a 5 s supervisor timeout (`go/internal/bridge/channel/e2e_test.go:45-59`) |
| `cmd/evolve` `TestSyncMain_TheLoopsInboxStampsNeverBlockTheSync` | 0.34, 0.37, 0.50, 0.63, 0.85 s | 1.00, 1.21, 1.63, 2.43, 2.66 s | parallel; it ran while the u23 CPU was 99% busy with a run queue of 10 |

*Implication:* `ClaudeSpan` varies 5× across runs and runs inside the most CPU-bound package (`bridge`). Its state machine depends on the order of two goroutines and a file. `TestChannel_EndToEnd` never comes near its 5 s timeout, so its flake is an order race, not a slow run (synthesis). Neither flake failed in these six runs, so this data cannot reproduce them.

**F3.32** C3 (PR #935, run 39033417060) cut the ubuntu suite by a third. That u23 job ran on the fast mode of F3.20, so the fair base is the C0b run:

| Measure | main median (4 runs) | C0b run (fast runner) | PR #935 |
|---|--:|--:|--:|
| u23 test step | 607 s | 432 s | 399 s |
| u27 test step | 609 s | 606 s | 512 s |
| mac test step | 1,100 s (5 runs, 815–1,208 s) | 1,168 s | 1,048 s |
| mac job | 1,386 s (5 runs, 1,069–1,463 s) | 1,523 s | 1,413 s |
| u23 Σ package wall | 1,612 s | 1,160 s | 1,064 s |
| `phases/audit` | 210 s | 135 s | 65.5 s |
| Σ of the `TestSkillsDriftGate_*` tests | 699 s | 460 s | 189 s |
| `TestAutoRespond_CodexUpdateMenuRulesDoNotMatchThisRepositorysOwnFiles` | 72 s | 32 s | 2.6 s |
| Σ of the `TransientDwell` tests | 102 s | 59 s | 167 s |

The macOS job of the PR run took 1,413 s, and its test step 1,048 s: inside the range of the main runs.
*Implication:* C3 and C5b work on ubuntu. On macOS, the gain is smaller than the run-to-run spread, because the macOS step is slot-bound and `phases/audit` is one of 301 packages (F3.21). The `TransientDwell` sum rose to 167 s in this one run. PR #935 does not change those tests, so the cause is the load beside them (synthesis), which a second run must confirm.

**F3.33** The race detector and coverage are not the whole macOS tax. The e2e step has no `-race`. In it, `cmd/evolve` still takes 162 s on macOS and 33 s on u23 (4.8×), with 203 s and 41 s of CPU.
*Implication:* removing `-race` on macOS cannot remove the kernel cost of F3.22. Scope (plan design A) stays the main macOS lever. The darwin-sensitive set of F3.10 is 901 of the 2,755 macOS package-seconds (33%), and 858 of the 2,288 CPU-seconds.

**F3.34** A test of the C1 report (`evolve ci telemetry`, branch `feat/ci-c1-critical-path`) found four defects. The report itself works. It names the L5 to L1 path correctly and splits the two tiers. It also matches the duplicate `cmd/*` rusage records to the right tier. One run takes 20 s; `--history 5` takes 2 min and downloads about 1.2 GB.

| Id | Defect | Evidence | Source |
|---|---|---|---|
| B1 | `cpu-saturation` uses the average of the whole step for every package. So all 303 u27 packages get `cpu-saturation` (step mean 91.5%). No u23 package gets it (mean 80.4%), but u23 was ≥ 95% busy for 57% of its seconds. | C0b run | `go/internal/citelemetry/path.go:267` |
| B2 | `system_s > user_s` has no lower bound. A binary with 0.011 s of CPU and 1 s of race sleep gets `subprocesses-and-syscalls`: 95 packages on u23 and 184 on macOS. Their real limit is the sleep of F3.26. | C0b run | `path.go:274` |
| B3 | `runnerCPUs` is a constant 4. The macOS runner has 3. | — | `path.go:19` |
| B4 | The floor model does not see contention. `core` on macOS has a floor of 92 s, a wall of 362 s and the label `dominant: serial`. But serial time is 15% of its wall. | C0b run | `p.Dominant()`, `path.go:225` |

The duration history also mixes red and green runs, while `cicd.pipeline.run.duration` groups by result (P4.6).
*Implication:* fix B1 and B2 before anyone acts on the `limit` field. Each needs a fixture test with the C0b numbers.

**F3.35** On macOS, the first start of each new executable can trigger a security assessment by `syspolicyd` (P4.10). Each package in the test step is a new test binary, and the e2e step builds new binaries again.
*Implication (synthesis, not measured):* this can add to the macOS kernel time of F3.22. It can apply to the binaries that the tests build, for example in `ciparity` and `phases/ship`. It cannot explain the `git` cost, because `git` is not a new binary. A measurement must come first.

## 4. Current practice

**P4.1** gotestsum's `tool ci-matrix` reads past `go test -json` timings and writes a GitHub Actions matrix that balances packages across jobs ([gotestsum](https://github.com/gotestyourself/gotestsum)). Its `tool slowest` finds slow tests from the same stream.

**P4.2** Depot splits a `go list ./...` package list across matrix jobs by timing ([Depot, split tests](https://depot.dev/docs/ci/how-to-guides/split-tests.md)). EdgeDB sharded its suite across GitHub Actions jobs for a 10× faster run ([EdgeDB](https://www.geldata.com/blog/how-we-sharded-our-test-suite-for-10x-faster-runs-on-github-actions)). A Vitest case study cut one 15-minute job to 4.5 minutes, building the shared artifact once ([Contentstack](https://developers.contentstack.com/blog/the-waiting-tax-on-every-pr-how-sharding-vitest-cut-ci-from-15-minutes-to-4-5)).

**P4.3** macOS minutes cost about 10× Linux minutes. A job that passes on Linux gains nothing from macOS ([dev.to](https://dev.to/ace2932/your-github-actions-bill-is-mostly-macos-minutes-heres-how-to-tell-56fk)). Larger macOS runners exist ([WarpBuild](https://warpbuild.com/blog/macos-runners)). A runner image update once made one macOS test step 2.5× slower ([runner-images #11845](https://github.com/actions/runner-images/issues/11845)).

**P4.4** The race detector adds a fixed cost per package and a large factor on long CPU-bound tests ([golang/go #20364](https://go.dev/issues/20364), [#54630](https://golang.org/issue/54630)). F3.9 shows that this suite is not CPU-bound under race.

**P4.5** Test impact analysis skips tests whose inputs did not change. Safe use keys on every input. It falls back to a full run when the dependency data is incomplete, and it keeps a scheduled full run ([QASkills](https://qaskills.sh/blog/ci-test-impact-caching-strategy), [Symflower](https://symflower.com/en/company/blog/2024/test-impact-analysis/), [Datadog](https://docs.datadoghq.com/ko/getting_started/test_impact_analysis/)). Go's own test cache keys on file mtimes, so it misses on CI ([golang/go #58571](https://golang.org/issue/58571)).

**P4.6** OpenTelemetry defines CI/CD semantic conventions. `cicd.pipeline.run.duration` is a histogram in seconds, grouped by pipeline name, state and result. `cicd.pipeline.run.errors` counts the errors of a run. Both are in development status ([OTel CI/CD metrics](https://opentelemetry.io/docs/specs/semconv/cicd/cicd-metrics/), [Go `cicdconv`](https://godocs.io/go.opentelemetry.io/otel/semconv/v1.38.0/cicdconv)).

**P4.7** `getrusage(RUSAGE_CHILDREN)` counts only children that were waited for, and it includes their waited-for descendants. Peak RSS is a running maximum, so one wrapper process per binary keeps the numbers separate ([LKML getrusage maxrss](https://lkml.iu.edu/hypermail/linux/kernel/0901.0/01452.html), [LTP getrusage03](https://lists.linux.it/pipermail/ltp/2021-June/023230.html)).

**P4.8** CI observability guides name more indexes than duration. They are queue time (the wait for a runner), cache restore and save time, and the success rate per stage. They also name the failure rate per test, which marks a flaky test, and CPU and memory per runner ([Splunk](https://www.splunk.com/en_us/blog/learn/monitoring-ci-cd?p=83), [Tinybird](https://www.tinybird.co/blog/data-driven-ci-pipeline-monitoring-with-pytest), [Metabase](https://www.metabase.com/dashboards/ci-pipeline-health), [DEV](https://dev.to/_6638a39c349d7e9c85ee20/cicd-observability-build-metrics-test-analytics-deployment-tracking-and-dora-metrics-3b5b)).

**P4.9** The `GORACE` option `atexit_sleep_ms` has a default of 1000. It is the "amount of milliseconds to sleep in the main goroutine before exiting" ([Go race detector, Options](https://go.dev/doc/articles/race_detector#Options), read 2026-10-17). `GORACE="atexit_sleep_ms=0"` removes the sleep. The detector still checks all memory accesses before the exit.

**P4.10** On macOS, `syspolicyd` assesses each new executable at its first start, and it caches the result. A build that makes many new executables pays this cost many times ([Lix CI commit](https://git.lix.systems/jade/lix/commit/cfe6ea746c3b88887eafda3eaccedc654f61651f), [Michael Tsai, 2025-04-30](https://mjtsai.com/blog/2025/04/30/why-some-apps-sometimes-launch-extremely-slowly/)). The sources do not measure Go test binaries.

## 5. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Keep the `-json` stream of every run (P4.1) | Adopt | F3.11: the data exists and is thrown away. |
| Shard by measured timing (P4.1, P4.2) | Adopt later | It helps only after the single-package poles shrink (F3.4). |
| Run macOS only where the OS matters (P4.3) | Adopt | F3.1 and F3.10: macOS is the critical path, and 290 of 303 packages do not depend on the OS. |
| Drop `-race` on macOS (P4.4) | Reject as the main lever | F3.9: about 20% saved. Scope saves more. |
| A larger macOS runner (P4.3) | Reject for now | It costs money every run, and F3.10 removes most of the macOS work for free. |
| Per-package rusage through `go test -exec` (P4.7) | Adopt | F3.13: it shows system-CPU and contention, which time alone hides. |
| A runner sampler for utilization and saturation (P4.8) | Adopt | F3.15: rusage cannot see contention between packages. |
| OpenTelemetry CI/CD names (P4.6) | Adopt for names only | One vocabulary for later export. No collector is needed now. |
| Test impact analysis (P4.5) | Adopt later | It is the largest per-PR win, but it needs content keys for non-Go inputs. ADR-0134's `testkey` is the base. |
| `GORACE=atexit_sleep_ms=0` for the test binaries (P4.9) | Adopt | F3.26: 301 slot-seconds of sleep per job. The detector keeps every check before the exit. |
| Turn off the macOS `syspolicyd` assessment (P4.10) | Reject for now | F3.35: not measured here, and it changes the security state of the runner. Measure first. |

## 6. Refinements to adopt

- **R1** (F3.11, P4.1): upload `test-integration.json` and an e2e `-json` stream from every Go job. Add an L1–L5 critical-path report that names, for the slowest job, the step, the package, the serial chain and the longest parallel test.
- **R2** (F3.1, F3.3, F3.10): run the macOS job over the darwin-sensitive packages and the e2e tier only. Run the OS-independent coverage gates on ubuntu 1.23 only. macOS keeps cover-changed and cover-strict for the darwin set.
- **R3** (F3.6): make the skills-drift tests share one generator build per distinct source tree. Or check through a small binary, not the whole `evolve`.
- **R4** (F3.7): split `cmd/evolve`'s serial tests across processes, or move them to injected environments so that they can run in parallel.
- **R5** (F3.8): give the `bridge` dwell tests an injected clock. F3.25 replaces this: the tests run a simulated tick loop, so the fix is fewer ticks (C10).
- **R6** (F3.4, P4.2): after R3–R5, balance the ubuntu suite across shards from the R1 history.
- **R7** (F3.13, F3.14, P4.7): a `go test -exec` wrapper records rusage per test binary: wall, user and system CPU, peak RSS, block I/O and context switches.
- **R8** (F3.15, P4.8): one runner sampler per Go job records CPU busy, run-queue length, memory, swap and disk wait every second.
- **R9** (P4.6, P4.8): one CI index set, named after the OpenTelemetry CI/CD conventions, is computed per run and kept as history.
- **R11** (F3.27): the e2e step runs only the tests that the integration step cannot run: the `e2e`-tagged tests.
- **R12** (F3.26, P4.9): every `-race` test run sets `GORACE=atexit_sleep_ms=0`.
- **R13** (F3.25): the 500 ms deliverable grace is a value that a test can set, so a test of a missing file does not sleep.
- **R14** (F3.25, F3.28): the whole-tree AST scans parse the tree once per test binary, and they keep one package in memory at a time.
- **R15** (F3.23, F3.35): the macOS sampler records CPU busy, as its spec says. A spawn counter shows which tests start the most processes.
- **R16** (F3.34): the C1 report decides the limit of a package over that package's own time window, with a lower bound on CPU.
- **R17** (F3.30): vet and gofmt run once, on ubuntu 1.23, with `GOOS=darwin go vet` for the darwin files.
- **R18** (F3.25): the `bridge` tick-loop tests reach their state in fewer ticks. An injected clock (C5) cannot help them, because they do not wait.
