# CI wall time: where the 21–25 minutes go, measured from the test up (research, 2026-10)

> The plan that uses this research: [ci-wall-time-2026-10](../plans/ci-wall-time-2026-10.md). The design: [ci-telemetry-and-critical-path](../architecture/ci-telemetry-and-critical-path.md). The decision record: [ADR-0135](../architecture/adr/0135-ci-measures-itself-before-it-changes-its-shape.md). Earlier work on the same suite: [test-system-efficiency-2026-10](test-system-efficiency-2026-10.md) and its [plan](../plans/test-system-efficiency-2026-10.md).

Date: 2026-10-17. Method:

- job and step timings for six `required CI` runs on main (`gh run view --json jobs`);
- per-package times from the full log of run 38974705695;
- a local `go test -json` profile of the five slowest packages, and a race-cost run;
- a code read of the slowest tests, and a web survey.

Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

- [1. The request](#1-the-request)
- [2. The measurement model](#2-the-measurement-model)
- [3. Local evidence](#3-local-evidence)
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

**F3.8** The `TransientDwell` and `PaneParity` tests in `bridge` take 33–48 s each, and they drive a real tmux pane through a dwell timer.
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

## 4. Current practice

**P4.1** gotestsum's `tool ci-matrix` reads past `go test -json` timings and writes a GitHub Actions matrix that balances packages across jobs ([gotestsum](https://github.com/gotestyourself/gotestsum)). Its `tool slowest` finds slow tests from the same stream.

**P4.2** Depot splits a `go list ./...` package list across matrix jobs by timing ([Depot, split tests](https://depot.dev/docs/ci/how-to-guides/split-tests.md)). EdgeDB sharded its suite across GitHub Actions jobs for a 10× faster run ([EdgeDB](https://www.geldata.com/blog/how-we-sharded-our-test-suite-for-10x-faster-runs-on-github-actions)). A Vitest case study cut one 15-minute job to 4.5 minutes, building the shared artifact once ([Contentstack](https://developers.contentstack.com/blog/the-waiting-tax-on-every-pr-how-sharding-vitest-cut-ci-from-15-minutes-to-4-5)).

**P4.3** macOS minutes cost about 10× Linux minutes. A job that passes on Linux gains nothing from macOS ([dev.to](https://dev.to/ace2932/your-github-actions-bill-is-mostly-macos-minutes-heres-how-to-tell-56fk)). Larger macOS runners exist ([WarpBuild](https://warpbuild.com/blog/macos-runners)). A runner image update once made one macOS test step 2.5× slower ([runner-images #11845](https://github.com/actions/runner-images/issues/11845)).

**P4.4** The race detector adds a fixed cost per package and a large factor on long CPU-bound tests ([golang/go #20364](https://go.dev/issues/20364), [#54630](https://golang.org/issue/54630)). F3.9 shows that this suite is not CPU-bound under race.

**P4.5** Test impact analysis skips tests whose inputs did not change. Safe use keys on every input. It falls back to a full run when the dependency data is incomplete, and it keeps a scheduled full run ([QASkills](https://qaskills.sh/blog/ci-test-impact-caching-strategy), [Symflower](https://symflower.com/en/company/blog/2024/test-impact-analysis/), [Datadog](https://docs.datadoghq.com/ko/getting_started/test_impact_analysis/)). Go's own test cache keys on file mtimes, so it misses on CI ([golang/go #58571](https://golang.org/issue/58571)).

**P4.6** OpenTelemetry defines CI/CD semantic conventions. `cicd.pipeline.run.duration` is a histogram in seconds, grouped by pipeline name, state and result. `cicd.pipeline.run.errors` counts the errors of a run. Both are in development status ([OTel CI/CD metrics](https://opentelemetry.io/docs/specs/semconv/cicd/cicd-metrics/), [Go `cicdconv`](https://godocs.io/go.opentelemetry.io/otel/semconv/v1.38.0/cicdconv)).

**P4.7** `getrusage(RUSAGE_CHILDREN)` counts only children that were waited for, and it includes their waited-for descendants. Peak RSS is a running maximum, so one wrapper process per binary keeps the numbers separate ([LKML getrusage maxrss](https://lkml.iu.edu/hypermail/linux/kernel/0901.0/01452.html), [LTP getrusage03](https://lists.linux.it/pipermail/ltp/2021-June/023230.html)).

**P4.8** CI observability guides name more indexes than duration. They are queue time (the wait for a runner), cache restore and save time, and the success rate per stage. They also name the failure rate per test, which marks a flaky test, and CPU and memory per runner ([Splunk](https://www.splunk.com/en_us/blog/learn/monitoring-ci-cd?p=83), [Tinybird](https://www.tinybird.co/blog/data-driven-ci-pipeline-monitoring-with-pytest), [Metabase](https://www.metabase.com/dashboards/ci-pipeline-health), [DEV](https://dev.to/_6638a39c349d7e9c85ee20/cicd-observability-build-metrics-test-analytics-deployment-tracking-and-dora-metrics-3b5b)).

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

## 6. Refinements to adopt

- **R1** (F3.11, P4.1): upload `test-integration.json` and an e2e `-json` stream from every Go job. Add an L1–L5 critical-path report that names, for the slowest job, the step, the package, the serial chain and the longest parallel test.
- **R2** (F3.1, F3.3, F3.10): run the macOS job over the darwin-sensitive packages and the e2e tier only. Run the OS-independent coverage gates on ubuntu 1.23 only. macOS keeps cover-changed and cover-strict for the darwin set.
- **R3** (F3.6): make the skills-drift tests share one generator build per distinct source tree. Or check through a small binary, not the whole `evolve`.
- **R4** (F3.7): split `cmd/evolve`'s serial tests across processes, or move them to injected environments so that they can run in parallel.
- **R5** (F3.8): give the `bridge` dwell tests an injected clock.
- **R6** (F3.4, P4.2): after R3–R5, balance the ubuntu suite across shards from the R1 history.
- **R7** (F3.13, F3.14, P4.7): a `go test -exec` wrapper records rusage per test binary: wall, user and system CPU, peak RSS, block I/O and context switches.
- **R8** (F3.15, P4.8): one runner sampler per Go job records CPU busy, run-queue length, memory, swap and disk wait every second.
- **R9** (P4.6, P4.8): one CI index set, named after the OpenTelemetry CI/CD conventions, is computed per run and kept as history.
