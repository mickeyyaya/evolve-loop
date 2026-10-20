# Research: how to keep the latency fixes from regressing

- **Status:** complete, 2026-10-20.
- **Plan:** [latency-gates-2026-10](../plans/latency-gates-2026-10.md). **Decision:** [ADR-0144](../architecture/adr/0144-latency-is-gated-by-invariants-at-merge-and-by-trends-on-main.md).
- **Related:** [wave-latency-2026-10](wave-latency-2026-10.md) (the fixes), [ci-wall-time-2026-10](ci-wall-time-2026-10.md) (the CI fixes).
- **Method:**
  - live web searches on 2026-10-20;
  - the CI jobs API for the last 40 runs of `required.yml`;
  - a read of the test and gate tools of the repository.

## Table of contents

- [1. The request](#1-the-request)
- [2. What the fixes are](#2-what-the-fixes-are)
- [3. What the repository already has](#3-what-the-repository-already-has)
- [4. Current practice](#4-current-practice)
- [5. Limits](#5-limits)

## Terms used throughout

- **Hard gate:** a check that blocks a merge. It must give the same result on each run.
- **Soft gate:** a check that records a trend, warns and files an item. It never blocks a merge.
- **Invariant:** a property of the behavior that holds on every input, for example "no slot is idle while an admissible todo waits".
- **Change point:** a commit after which a measured series moves to a new level and stays there.

## 1. The request

The operator asked on 2026-10-20: "could you build a strong test plan to cover that performance fix and gate the latency to prevent it from regression?"

## 2. What the fixes are

| Fix | What it changed | Measured effect |
|---|---|---|
| #988 | a concurrency group for each PR, and a light route for inbox-only changes | macOS queue median 60 min to 0.2 min; run wall median 80 min to 22.4 min (40 runs, 2026-10-20) |
| #990 | the loopwave ledger no longer leaks, so a clean rebase keeps its audit | audit runs per cycle 1.83 to 0.95 |
| #991 | the pool takes new work after a lane exits | `LOOP_ORCH_POOL_RESUPPLIED` fired in wave 133 |
| #996 (W1) | admission holds only shared edits, at stage shadow | not yet measured |
| #998 (W5) | one verdict store for the floor and the ship, at stage shadow | not yet measured |
| #999 (W4) | the build seat orders families by headroom and lane time | not yet measured |
| C2 | macOS runs only the darwin package set | in build |

## 3. What the repository already has

**F3.1** `cmd/testlatency` parses `go test -json` and prints the wall time of each package. It has the flags `--fail-over-budget` and `--package-budget` (`go/cmd/testlatency/main.go:28-30`). The second flag applies `addedtests.PackageWallBudget` and exits 1 on a breach, and `make test-budget` uses it.

CI runs the tool as an advisory job summary (`.github/workflows/go.yml:74`). The tool keeps no baseline.
*Implication:* the parser and the fail path exist. A duration gate needs only a size class and a recorded ratchet.

**F3.2** `internal/sizeratchet` keeps a recorded list of offenders (`offenders.json`), fails a new or grown offender, and has a `Tighten` step that lowers a recorded value.
*Implication:* the ratchet pattern exists. A duration ratchet can follow it.

**F3.3** `internal/wavemeasure.Compute(Facts, now)` (`go/internal/wavemeasure/wavemeasure.go:50`) computes the wave measures from facts, with the clock passed in. `Facts` has no field for the gate share and no flag for a brake or a quota pause. The package `internal/lanetime` and `evolve wave latency` are pending W0 (PR #997) and are not on `main`.
*Implication:* the wave measures can be tested on fixtures with no real time. T6 must add the missing fields.

**F3.4** `fleet.RunPool` takes a `LaunchFn`, and the pool tests drive it with fakes and no sleeps.
*Implication:* a seeded simulation of a wave can drive the real pool and the real admission with no clock.

**F3.5** The module targets Go 1.23. `testing/synctest` needs Go 1.25.
*Implication:* the simulation uses the injected clocks and fakes, not synctest.

## 4. Current practice

### Measured gates on noisy data

**G1** MongoDB replaced fixed thresholds with change-point detection, which "dramatically dropped our false positive rate" ([arXiv 2003.00584](https://arxiv.org/abs/2003.00584), 2020).
*Implication:* a measured gate looks for a lasting shift, not one slow run.

**G2** Hunter, now Apache Otava, uses a t-test in its change-point search. The permutation test did not give deterministic results ([arXiv 2301.03034](https://arxiv.org/abs/2301.03034), 2023; [Otava](https://otava.apache.org), acc.).
*Implication:* the test of a gate must be deterministic. A random permutation is not allowed.

**G3** benchstat compares two sets with a Mann-Whitney U test and medians, with at least 10 and ideally 20 samples. It warns against a rerun until a result turns significant ([benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat), acc.).

**G4** On cloud machines the coefficient of variation of a benchmark goes from 0.03% to more than 100%. A rank-sum test across different instances gave 26% to 81% false positives (Laaber and others, EMSE 2019, [Chalmers](https://research.chalmers.se/publication/511491); search summary).
*Implication:* a wall-time check on a CI runner cannot block a merge.

**G5** A significant change can be too small to matter, so gates also need a minimum effect size ([arXiv 2405.13528](https://arxiv.org/abs/2405.13528), 2024). Mozilla Perfherder alerts on a confidence and a magnitude threshold together ([arXiv 2503.16332](https://arxiv.org/abs/2503.16332), 2025).
*Implication:* a measured gate needs a relative threshold and an absolute floor.

**G6** Bencher uses a t-test when the history has fewer than 30 points, and keeps a window of history, for example 4 weeks ([Bencher](https://bencher.dev/docs/explanation/thresholds), acc.).

**G7** CodSpeed counts instructions in a simulated CPU, so the measure does not depend on the runner ([CodSpeed](https://codspeed.io/docs/instruments/cpu), acc.).
*Implication:* where possible, count work (allocations, test runs, dispatches) in place of time.

### Deterministic tests of throughput

**G8** synctest gives a fake clock that moves only when all goroutines block, so time tests are fast and not flaky ([Go blog](https://go.dev/blog/synctest), 2025-02-19). FoundationDB and TigerBeetle drive the clock, the network and the disk from one seeded generator, so a failed seed replays exactly ([VOPR](https://docs.tigerbeetle.com/about/vopr), acc.).
*Implication:* the pool and admission behavior can be a hard gate when a seeded simulation tests it.

**G9** The Kubernetes scheduler measures throughput in benchmarks and checks correctness in a simulator: two separate tiers ([Preferred Networks](https://tech.preferred.jp/en/blog/kubernetes_scheduler_benchmark/), search summary).

### Duration budgets for tests

**G10** Bazel gives each test a size class with a coarse timeout: 60 s, 300 s, 900 s and 3600 s. It warns when a test is much faster than its class ([Bazel](https://bazel.build/reference/test-encyclopedia), acc.).
*Implication:* a test-duration gate uses coarse classes with large headroom, not a tight budget for each test.

**G11** A per-test timeout for Go was accepted but is not merged ([golang/go#48157](https://github.com/golang/go/issues/48157), read 2026-10-20).

### Alerts on pipelines

**G12** The SRE workbook alerts when both a long and a short window burn the error budget. It warns that a low volume of events needs long windows ([SRE workbook](https://sre.google/workbook/alerting-on-slos/), acc.).
*Implication:* CI runs are a low-volume series, so a CI alert needs several runs and a consecutive-breach rule.

## 5. Limits

- The jobs API has no field for the queue time. This research computes it as `started_at` minus `created_at`.
- Several sources were read only as search summaries. The text marks them.
