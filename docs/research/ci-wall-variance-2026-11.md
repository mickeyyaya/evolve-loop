# CI wall variance: why the package wall time of the same tests changes by 2.3 times (research, 2026-11)

- **Status:** complete, 2026-11-01. Section 5.1 (the share of each component) and the hard ratchet were added on 2026-11-14, cycle 2360.
- **Inbox item:** `ci-wall-variance-deep-dive`.
- **Related:** [ci-wall-time-2026-10](ci-wall-time-2026-10.md), [wave-latency-2026-10](wave-latency-2026-10.md).
- **Method:** the CI artifacts of five runs, code reads with `path:line`, and live web searches on 2026-11-01 (section 4.3).

## Table of contents

- [1. Purpose](#1-purpose)
- [2. Scope](#2-scope)
- [3. Method](#3-method)
- [4. Findings](#4-findings)
- [5. Causes ranked](#5-causes-ranked)
- [6. Recommendations](#6-recommendations)
- [7. Limits](#7-limits)

## Terms used throughout

- **Host class:** the hardware of one macOS CI job. The runs show two classes: 3 cores and 5 cores.
- **Package CPU:** `user_s + system_s` of one test binary in `test-rusage.ndjson`. The value comes from `wait4`, through `ProcessState.SysUsage` (`go/cmd/testrusage/rusage_unix.go:10`).
- **Speed index (S):** the median, over the reference packages, of the package CPU of a run divided by the package CPU of the baseline run.
- **Normalized CPU (N):** the package CPU divided by S.
- **Reference package:** a package with 2 s or more of package CPU in the baseline run.
- **Baseline run:** run 40582532714, unless a table names another run.

## 1. Purpose

The operator asked on 2026-11-01: "Remove the wall time to investigate why, deep dive. Find which phase or component is causing the flaky result."

The package wall ratchet in `.github/workflows/go.yml` was report-only for this investigation. This document finds the cause of the wall variance. It also gives a ratchet judgment that holds on a noisy runner. Cycle 2360 implemented that judgment, and both CI ratchet steps fail a regression again (section 6.3).

## 2. Scope

This research covers these items:

- the job `build + test (Go 1.23, macOS, the darwin package set)`, which tests the 18 or 19 packages of `go/.darwin-set`;
- the job `build + test (Go 1.23, ubuntu, every package and every gate)`, which tests about 355 packages;
- the four packages with the largest swing: `internal/bridge`, `cmd/evolve`, `internal/core` and `internal/phases/ship`, and `internal/phases/audit` on ubuntu.

This research does not change code or CI files. It does not cover the e2e tier or the Go 1.27 job.

## 3. Method

1. Download `coverage-macos-latest-go1.23` and `coverage-ubuntu-latest-go1.23` of each run with `gh run download <id> -n <artifact>`.
2. Read the package `Elapsed` and the top-level test `Elapsed` from `test-integration.json`.
3. Read the package CPU, the peak RSS and the context switches from `test-rusage.ndjson`.
4. Read the host load from `runner-samples.ndjson`. The macOS sampler writes memory only. It writes no CPU field.
5. Compute the start and the end of each package from the `Time` of its first and last JSON event.
6. Read the runner image and the region from the header of each job log (`gh run view --job <id> --log`).
7. Read the code of the top swingers. Classify each one by what it waits on.
8. Replay the current wall rule and the proposed CPU rule over the five runs.

The five runs:

| Run | Head | Branch | macOS image | macOS job |
|---|---|---|---|---|
| 40526866678 | `eff18a9d7` | `main` | `macos-26-arm64` 20261021.0445.8 | try 1 |
| 40563673621 | `2ae0a7eeb` | PR #1223 (the quadratic-parse fix) | `macos-26-arm64` 20261013.0424.1 | try 1 |
| 40576371034 | `14dc7eaf6` | PR #1225 (fewer git starts in ship) | `macos-26-arm64` 20261021.0445.8 | try 2 |
| 40582532714 | `9e57fd839` | PR #1226, first push | `macos-26-arm64` 20261013.0424.1 | try 1 |
| 40611567029 | `4261a2edd` | PR #1226, later push | `macos-26-arm64` 20261013.0424.1 | try 2 |

**F3.1** The two PR #1226 runs do not test the same code. Run 40582532714 tests `9e57fd839`, and run 40611567029 tests `4261a2edd`. The later head adds the package `internal/phases/ship/shiptest` and moves 84 tests out of `internal/phases/ship`.
*Implication:* a part of the ship difference is a code change. The bridge and `cmd/evolve` test lists are equal (1536 and 3238 tests in both runs).

## 4. Findings

### 4.1 The macOS host class

**F4.1** The macOS jobs ran on two host classes. `go test` sets `-p` to the number of CPUs, so the class sets how many test binaries run at the same time.

| Run | Test binaries at the same time (max) | Cores busy with tests (mean) | Peak memory used | Test span | bridge wall | `cmd/evolve` wall | core wall | ship wall |
|---|---|---|---|---|---|---|---|---|
| 40526866678 | 3 | 2.26 | 4274 MB | 615 s | 604 s | 489 s | 298 s | 130 s |
| 40563673621 | 5 | 3.96 | 7706 MB | 322 s | 314 s | 307 s | 233 s | 122 s |
| 40576371034 | 3 | 2.32 | 4318 MB | 677 s | 656 s | 484 s | 497 s | 125 s |
| 40582532714 | 5 | 4.12 | 7723 MB | 339 s | 331 s | 339 s | 266 s | 131 s |
| 40611567029 | 3 | 2.27 | 4398 MB | 903 s | 748 s | 781 s | 544 s | 165 s |

"Cores busy" is the sum of the package CPU divided by the test span. A mean of 4.12 cores is not possible on a 3-core host. The 7.7 GB of used memory is not possible on a 7 GB host.

*Implication:* the fast runs had a host with at least 5 cores and more than 7 GB of memory. The slow runs had the 3-core host. The standard arm64 macOS runner has 3 M1 cores and 7 GB. The XLarge runner has 5 cores and 14 GB (section 4.3).

**F4.2** On the 3-core host, `cmd/evolve`, `internal/bridge` and `internal/core` run at the same time and fill the 3 slots. Each binary also runs its `t.Parallel` tests with `GOMAXPROCS=3`. Thus each package gets about 0.75 core.

On the 5-core host, `internal/phases/ship` and `internal/looppreflight` start in the first 26 s. On the 3-core host, ship starts only when one of the three large packages ends: at 364 s, 523 s and 738 s.
*Implication:* the host class changes the wall of each large package and the order of the packages.

- On the PR #1226 pair, the wall ratio is 2.0 to 2.3: bridge 2.26, `cmd/evolve` 2.30, core 2.05 (F4.1).
- The ratio is the wall of run 40611567029 divided by the wall of run 40582532714.
- This pair also differs in CPU speed: S is 1.46 against 1.00 (F4.4).
- Two pairs with an equal S give 1.3 to 1.9 for the class alone:
  - runs 40526866678 and 40563673621: `cmd/evolve` 1.59, core 1.28;
  - runs 40576371034 and 40582532714: `cmd/evolve` 1.43, core 1.87.

**F4.3** The image version does not decide the class. Run 40611567029 had the image 20261013.0424.1, which the two fast runs also had, but it ran on the 3-core host.
*Implication:* a pin of the image does not remove the variance. Only a pin of the hardware or a judgment that ignores the hardware removes it.

### 4.2 Wall against CPU

**F4.4** The speed index changes from run to run on both OSes. The index compares the package CPU of each run with the baseline run.

| Run | macOS S (4 reference packages) | ubuntu S (39 to 40 reference packages) | ubuntu region |
|---|---|---|---|
| 40526866678 | 0.90 | 1.52 | eastus |
| 40563673621 | 0.91 | 1.26 | centralus |
| 40576371034 | 1.02 | 1.29 | westus3 |
| 40582532714 | 1.00 | 1.00 | centralus |
| 40611567029 | 1.46 | 1.06 | westus3 |

*Implication:* one CPU second of work costs from 0.90 to 1.52 CPU seconds, by the host. The same tests use 46% more CPU on the host of run 40611567029 than on the host of run 40582532714.

**F4.5** Normalized CPU is stable where wall and raw CPU are not. The table gives the max/min ratio over the clean runs. A clean run has no known regression. For `internal/bridge`, the clean runs are 40563673621, 40582532714 and 40611567029. For the other packages, all five runs are clean. N uses a leave-one-out S: the judged package is not a reference package for its own S.

| OS | Package | Wall max/min | CPU max/min | N max/min |
|---|---|---|---|---|
| macOS | `internal/bridge` | 2.38 | 1.57 | 1.03 |
| macOS | `cmd/evolve` | 2.54 | 1.72 | 1.18 |
| macOS | `internal/core` | 2.33 | 1.63 | 1.18 |
| macOS | `internal/phases/ship` | 1.35 | 1.40 | 1.38 (code change, F3.1 and F4.9) |
| ubuntu | `cmd/evolve` | 1.49 | 1.50 | 1.06 |
| ubuntu | `internal/bridge` | 1.39 | 1.36 | 1.14 |
| ubuntu | `internal/phases/audit` | 1.50 | 1.44 | 1.15 |
| ubuntu | `internal/core` | 1.52 | 1.42 | 1.25 |
| ubuntu | `internal/phases/ship` | 1.57 | 1.62 | 1.18 |
| ubuntu | `test/structure` | 1.70 | 1.91 | 1.26 |

*Implication:* the host explains almost all of the wall swing. On the PR #1226 pair, bridge wall went from 331 s to 748 s, but bridge N went from 390 s to 385 s.

**F4.6** The real regression of run 40526866678 is clear in N. `TestDriverNames_AreBuiltOnlyByResolve` ran 433 s on ubuntu and 298 s on macOS. PR #1223 fixed the quadratic parse. Run 40576371034 has the same defect (345 s ubuntu, 390 s macOS), because PR #1225 branched before the fix.

| OS | Bridge N, clean runs | Bridge N, defect runs | Defect / clean median |
|---|---|---|---|
| macOS | 385 s to 395 s | 579 s and 594 s | 1.48 and 1.52 |
| ubuntu | 155 s to 176 s | 329 s and 331 s | 2.01 and 2.00 |

*Implication:* a threshold between 1.26 (the largest clean increase) and 1.48 (the smallest defect increase) separates the two groups.

**F4.7** macOS spends much more time in the kernel. The ratio `system_s / user_s` of each package:

| Package | macOS (run 40611567029) | ubuntu (run 40611567029) |
|---|---|---|
| `internal/core` | 1.76 | 0.91 |
| `internal/phases/ship` | 1.31 | 0.48 |
| `cmd/evolve` | 0.77 | 0.24 |
| `internal/bridge` | 0.28 | 0.10 |

The macOS jobs also have 4.7 to 6.0 million involuntary context switches for 18 packages. The ubuntu jobs have 2.3 to 3.0 million for about 355 packages.
*Implication:* process starts and file I/O cost more on macOS. On a 3-core host, this kernel time competes with the test CPU.

### 4.3 Current practice

- **P4.1** GitHub documents the standard arm64 macOS runner as 3 M1 cores and 7 GB of memory. The XLarge arm64 runner has 5 cores and 14 GB ([GitHub-hosted runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners), [larger runners](https://docs.github.com/en/actions/reference/runners/larger-runners)). The fast class of F4.1 matches the XLarge size. Synthesis: the `macos-latest` label sometimes gets the larger hardware.
- **P4.2** Wall-time noise on shared CI runners can be as large as the regressions to find. Instruction counts moved about 0.035% under contention, while wall medians moved about 150% ([tak experiment](https://tak.jdx.dev/guide/experiment), [tak-cli](https://docs.rs/crate/tak-cli/0.0.7)).
- **P4.3** The rustls CI benchmark gates on instruction counts from Cachegrind and keeps wall time as a report ([rustls ci-bench](https://github.com/rustls/rustls/blob/main/ci-bench)). The practice also says: keep a separate series for each runner class ([tak experiment](https://tak.jdx.dev/guide/experiment)).
- **P4.4** Consistent benchmarks in CI need a stable metric, not a stable host ([pythonspeed](https://pythonspeed.com/articles/consistent-benchmarking-in-ci)).

*Implication:* the house data agrees with current practice. A CPU-based metric, normalized for each run, is the right gate. Wall time is a report.

### 4.4 The top swingers and what they wait on

The table compares the PR #1226 pair on macOS (run 40582532714, then run 40611567029), unless the row says otherwise.

| Package | Test | Wall | What it waits on | Evidence |
|---|---|---|---|---|
| bridge | `TestAgentSandboxContract_TheHostHoldsTheCredentialThatTheCellCannotRead` | 41.7 s, 97.9 s | process starts: `sandbox-exec`, real tmux, `security`, and a `go build` of `./cmd/evolve` | `go/internal/bridge/sandbox_contract_integration_test.go:340`, `:493` |
| bridge | `TestAgentSandboxContract_TheSelfhealOrchestratorSeatHoldsN3AndN4` | 26.8 s, 91.7 s | the same, with its own `go build` | `sandbox_contract_integration_test.go:630` |
| bridge | `TestRunTmuxREPL_TransientDwell_ShadowObservesWithoutActing` | 25.0 s, 83.8 s | CPU and file writes under contention; the tmux and the sleep are fakes | `go/internal/bridge/transient_dwell_shortcircuit_test.go:32` |
| bridge | `TestPaneParity_RealTmuxPackagesPassInTheSandboxedPaneAsOnTheHost` | 30.8 s, 55.8 s | a nested `go test` on the host and in a sandboxed real tmux pane | `go/internal/bridge/pane_parity_integration_test.go:177` |
| bridge | `TestSelfMatchGuard_ThePrefilterGivesTheExhaustiveVerdictForEveryRule` | 14.1 s, 35.0 s | CPU under contention | test body |
| bridge | `TestDriverNames_AreBuiltOnlyByResolve` (runs 40526866678 and 40576371034) | 298 s, 390 s | CPU: the quadratic parse, fixed by PR #1223 | F4.6 |
| bridge | `TestTryRecord_AnR1TryStartsANewSessionAndRecordsTheDecision` (all runs) | 2.5, 63.3, 121.7, 4.6, 3.6 s | a real 60 s deadline: the poll of `/usr/bin/log show` for the sandbox fence | F4.8 |
| `cmd/evolve` | `TestPRRefresh_TheRealGeneratorsOfACloneCommitOnlyTheStaleIndex` | 61.1 s, 156.2 s | process starts and I/O: a local `git clone` of the repository, then the generators. Git hardlinks the objects, so the checkout probably costs more than the clone (not measured) | `go/cmd/evolve/cmd_pr_refresh_integration_test.go:21` |
| `cmd/evolve` | `TestInboxWriters_EveryAutofiledItemCarriesAKnownClass` | 7.5 s, 92.7 s | CPU: a scan of the Go tree, in the parallel queue | `go/cmd/evolve/inbox_writers_contract_test.go:80` |
| `cmd/evolve` | `TestRatchet_CheckPassesOnTheRealModuleAndRejectsOtherVerbs` | 31.4 s, 70.1 s | CPU: a size check over the real module | `go/cmd/evolve/cmd_ratchet_test.go:35` |
| `cmd/evolve` | `TestUnobservedLedgerRootsArePinned` and the other "pinned" tests | 3 s to 12 s, then 21 s to 37 s each | CPU and I/O: each test walks and reads the module again | `go/cmd/evolve/cmd_cycle_ledger_roots_test.go:16`, its walk at `:30` |
| core | (whole package) | 266 s, 544 s | process starts: `system_s` is 1.76 times `user_s`; 86 of 591 test files start git (note 1) | F4.7 |
| ship | (whole package) | 131 s, 165 s | process starts: 18 of 148 test files start git directly (note 1) | F4.7, F4.9 |
| audit (ubuntu) | the slow `TestSkillsDriftGate_*` tests | 19 s to 74 s each | compile: a copy of the module in a git fixture, then a generator build | `go/internal/phases/audit/audit_skillsdrift_worktree_integration_test.go:121` |

Note 1: the count is of the `_test.go` files in the package folder that match `gittest\.|exec\.Command(Context)?\([^)]*"git"|"git",`, at commit `833dac0db`. A file that starts git through a helper in another file is not in the count.

**F4.8** The bridge engine wires the real macOS log reader and the real fence by default (`go/internal/bridge/engine.go:224`, `:227`). On darwin, a launch with a worktree writes a fence with `sandbox-exec`. Then it polls `log show` until the fence message shows, or until a real 60 s deadline (`go/internal/bridge/read_isolation.go:39`, `:212`). Many tests inject a no-op `Sleep`, so the poll starts `log show` in a tight loop.

`TestTryRecord_*` does two launches and took 121.7 s, which is 2 × 60 s. `TestLaunchArgs_ProjectsProfileWriteSubpathsToTheWrapper` does one launch and took 61.1 s. No ubuntu test shows this pattern, because the reader applies on darwin only (`read_isolation.go:252`).
*Implication:* this is a true flake on macOS. It adds 60 s for each launch when the unified log is late. It also starts `log show` processes in every sandboxed launch of a unit test.

**F4.9** A real code change shows in N, as it must. PR #1225 cut the git process starts of ship. Ship N on macOS went from 226 s (run 40526866678) to 177 s (run 40576371034), which is 22% less.
*Implication:* N tracks real work. A fix that removes process starts shows as a lower N.

**F4.10** The current wall rule fails on the wrong evidence. The rule fails when the wall is above `min(2 × class ceiling, TEST_TIMEOUT)` (`go/cmd/testlatency/ratchet.go:50`). `internal/phases/ship` has the class `small`, so its limit is 120 s. The macOS ship wall is 122 s to 165 s in all five runs.
*Implication:* on macOS, the ship row fails in every run. The class records fit the ubuntu job only.

### 4.5 The replay of the two rules

The proposed rule is in section 6.2: fail when N > 1.35 × the baseline CPU and N − the baseline CPU > 15 s. The baseline is run 40582532714.

| Run | macOS, current wall rule | macOS, proposed rule | ubuntu, current wall rule | ubuntu, proposed rule |
|---|---|---|---|---|
| 40526866678 (defect) | FAIL: ship 130 > 120, bridge 604 > 600 | FAIL: bridge 1.48× | FAIL: audit 126 > 120 (not the defect) | FAIL: bridge 2.01× |
| 40563673621 (clean) | FAIL: ship 122 > 120 | pass (worst 1.09×) | pass | pass (worst 1.32×, below the floor) |
| 40576371034 (defect) | FAIL: ship 125 > 120, bridge 656 > 600 | FAIL: bridge 1.52× | pass (misses the defect) | FAIL: bridge 2.00× |
| 40582532714 (clean) | FAIL: ship 131 > 120 | pass (baseline) | pass | pass (baseline) |
| 40611567029 (clean) | FAIL: bridge 748, `cmd/evolve` 781, ship 165 | pass (worst 1.29×, `internal/looppreflight` under the floor) | pass | pass (worst 1.11×) |

The result does not depend on the choice of the baseline run:

- With baseline 40563673621, the proposed rule fails only the two defect runs: bridge 1.50× and 1.53× on macOS, and 1.85× and 1.92× on ubuntu.
- With baseline 40611567029, it also fails ship in run 40582532714 (1.38×). That run is older code with 84 more ship tests (F3.1). A baseline moves forward with `main` only, so this case does not occur in CI.

*Implication:* the proposed rule passes the three clean runs and fails the two defect runs on both OSes. The current wall rule fails macOS in all five runs and misses the ubuntu defect of run 40576371034.

## 5. Causes ranked

### 5.1 The share of each component in the wall variance

The table ranks the components of the wall variance of `internal/bridge` on macOS across the five runs. Bridge is the package of the incident: 331 s and 748 s on the same code.

Method (synthesis, from the values of section 4):

1. Take the log of the bridge wall of each run (604 s, 314 s, 656 s, 331 s, 748 s). Subtract the mean.
2. Split each value into four parts. Wall scales with CPU time, so each part is a log factor.
3. The CPU speed part is the log of S for macOS (F4.4). The regression part is the log of the defect increase (1.48 and 1.52) in runs 40526866678 and 40576371034.
4. The host class part is the mean of what remains for the 3-core runs and for the 5-core runs. The last part is the rest.
5. The share of a part is its covariance with the wall, divided by the variance of the wall. The four shares sum to 100%.

| Rank | Component | Share | Waits on | Evidence |
|---|---|---|---|---|
| 1 | The macOS host class: 3 cores or 5 cores | 41.3% | CPU slots: `go test -p` sets the number of binaries that run at the same time, so a 3-core host gives each package 0.75 core | The slow runs 40526866678, 40576371034 and 40611567029 ran on 3 cores. `TestAgentSandboxContract_TheHostHoldsTheCredentialThatTheCellCannotRead` took 41.7 s and 97.9 s (F4.1, F4.2) |
| 2 | A real regression: the quadratic parse | 28.9% | CPU: the parse of the driver names | `TestDriverNames_AreBuiltOnlyByResolve` took 298 s and 390 s in runs 40526866678 and 40576371034. PR #1223 fixed it (F4.6) |
| 3 | The CPU speed of the host | 27.7% | CPU: one CPU second of work costs 0.90 to 1.46 CPU seconds, by the host | S of run 40611567029 is 1.46. `TestSelfMatchGuard_ThePrefilterGivesTheExhaustiveVerdictForEveryRule` took 14.1 s and 35.0 s (F4.4) |
| 4 | The waits of the tests themselves | 2.1% | process starts (`sandbox-exec`, real tmux, `go build`, git), the real 60 s deadline of the `log show` poll, and disk I/O | `TestTryRecord_AnR1TryStartsANewSessionAndRecordsTheDecision` took 2.5 s, 63.3 s, 121.7 s, 4.6 s and 3.6 s. `TestPaneParity_RealTmuxPackagesPassInTheSandboxedPaneAsOnTheHost` took 30.8 s and 55.8 s (F4.8, section 4.4) |

*Implication:* the host, not a test, makes the swing. Rank 1 and rank 3 are 69.0% of the variance. A fix of the tests alone cannot remove them. A CPU judgment that divides by S removes rank 3. A fixed schedule removes most of rank 1 (R3).

Rank 4 is small because the tests that wait on the clock or on a process start are few. They can still add 60 s to 120 s to one run. Two of the five runs had such a wait. The share of rank 4 is the part of the spread that they explain. It is not the size of one wait.

Cross-check: the same split on two packages with no regression gives a similar order.

| Package | Host class | CPU speed | Regression | The rest |
|---|---|---|---|---|
| `cmd/evolve` (walls 489 s, 307 s, 484 s, 339 s, 781 s) | 53.3% | 43.5% | 0.0% | 3.2% |
| `internal/core` (walls 298 s, 233 s, 497 s, 266 s, 544 s) | 46.0% | 39.6% | 0.0% | 14.4% |

The rest of core is 14.4%. Core has `system_s` at 1.76 times `user_s` on macOS (F4.7), so its rest is mainly process starts: 86 of 591 test files start git.

Limits of the split: the sample has five runs, and wall scales with CPU time in this model. The split gives an order of the components, not exact numbers.

### 5.2 Causes by the size of the wall effect

The rank of this table is by the size of the wall effect in the evidence.

| Rank | Cause | Evidence | Size |
|---|---|---|---|
| 1 | The macOS host class: 3 cores or 5 cores | F4.1, F4.2, F4.3 | 1.3 to 1.9 times the wall of a large package at an equal S. With the CPU speed of rank 2, 2.0 to 2.3 times (F4.2). The span is 322 s to 339 s against 615 s to 903 s |
| 2 | The CPU speed of the host, on both OSes | F4.4 | S from 0.90 to 1.46 on macOS, and from 1.00 to 1.52 on ubuntu |
| 3 | A real regression: the quadratic parse in `TestDriverNames_AreBuiltOnlyByResolve` | F4.6 | 298 s to 433 s in one test; N up 1.48 to 2.01 times |
| 4 | Process starts on macOS (git, `go build`, `go test`, `sandbox-exec`, tmux, `log show`), which grow with contention | F4.7, section 4.4 | `system_s` above `user_s` in core and ship. The three sandbox and pane tests of section 4.4 that start processes take 1.8 to 3.4 times the wall |
| 5 | A real 60 s deadline in the bridge fence poll on macOS | F4.8 | 0 to 2 tests in each macOS run; +60 s for each late launch |
| 6 | Repeated scans of the whole module in `cmd/evolve` tests | section 4.4 | 36 scan sites in 23 test files; 3 s to 37 s each, by contention |

No single phase or component causes the macOS swing of PR #1226. The bridge, `cmd/evolve` and core all slowed by 2.0 to 2.3 times on the same host (F4.2). Bridge N went from 390 s to 385 s on this pair (F4.5). Over the clean runs, the N max/min of `cmd/evolve` and core is 1.18 (F4.5). Ranks 4 to 6 make the packages heavy and sensitive to the host. Ranks 1 and 2 decide the swing.

## 6. Recommendations

Each fix has a filed inbox item in the column Follow-up. The inbox item `ratchet-speed-adjusted-cpu` holds the decision on section 6.2. Cycle 2360 implemented it (section 6.3).

### 6.1 Fixes for each component, ranked by the variance that they remove

| Id | Fix | Variance removed | Evidence | Follow-up |
|---|---|---|---|---|
| R1 | Gate on N, not on wall (section 6.2). Keep the wall as a report and as the `TEST_TIMEOUT` cap. | All of ranks 1 and 2 from the verdict | F4.5, section 4.5 | `ratchet-speed-adjusted-cpu`, done in cycle 2360 |
| R2 | Record the host class in `runner-samples.ndjson`: `hw.ncpu` and `hw.memsize` on macOS, `nproc` on Linux. Keep one baseline series for each OS and class. | Makes rank 1 visible in each report | F4.1, P4.3 | `ci-runner-host-class-recorded` |
| R3 | Make the macOS schedule equal on both classes: run the darwin set with a fixed `-p 3`, or pin the job to the XLarge label. The XLarge label costs money. | Most of rank 1 in the wall report | F4.2 | `ci-macos-fixed-parallelism` |
| R4 | In bridge unit tests, inject fakes for `SandboxReports` and `SandboxFence` in `newTestEngine` (`go/internal/bridge/launch_test.go:121`). Keep the real `log show` only in the sandbox integration tests. | Rank 5: up to 120 s in one test; fewer `log show` starts | F4.8 | `bridge-test-engine-fakes-sandbox-fence` |
| R5 | Build `./cmd/evolve` once for each bridge test binary, for example with `sync.OnceValue`. Today three tests build it (`sandbox_contract_integration_test.go:278`, `:340`, `:630`). Each build puts the dispatch folder of its test into the binary with `-X …dispatchcred.root` (`:491-493`). Thus a shared build needs one shared dispatch root, or a root that the binary gets at run time. | A part of rank 4 in bridge | section 4.4 | `bridge-tests-build-evolve-once` |
| R6 | In `cmd/evolve`, read the module once for each test binary and share the file list, the bytes and the ASTs. Today 23 test files walk or parse it again. | Rank 6 | section 4.4 | `cmd-evolve-tests-read-module-once` |
| R7 | In `TestPRRefresh_*`, first measure the checkout and the generators apart. `gittest.Clone` (`go/internal/gittest/fixture.go:73`) clones a local path, so git ignores `--depth 1` and hardlinks the objects. Then reuse a `gittest.FromTemplate` fixture, or run fewer generators. | A part of 61 s to 156 s of serial time in `cmd/evolve` | section 4.4 | `pr-refresh-test-measure-checkout` |
| R8 | Cut the git process starts of core, as PR #1225 did for ship. | A part of rank 4; ship N fell 22% | F4.7, F4.9 | `core-tests-cut-git-starts` |
| R9 | Give the macOS ship row its own record, or class it `medium`. | Removes the ship failure of every macOS run | F4.10 | `ratchet-speed-adjusted-cpu`, closed: the CPU judgment drops the class wall limits |
| R10 | Seed the CPU baselines of both OSes from one clean main run. The gate fails nothing until the file has them. | The gate bites | section 6.3 | `ratchet-cpu-baseline-seed` |

### 6.2 The proposed ratchet judgment

The judgment uses the package CPU from `test-rusage.ndjson`. Each OS has its own baseline file. The baseline CPU of package p is `C(p)`.

1. For each package p, compute S over the reference packages other than p. S is the median of `cpu_run(q) / C(q)`.
2. Compute `N(p) = cpu_run(p) / S`.
3. Fail p when `N(p) > 1.35 × C(p)` and `N(p) − C(p) > 15 s`.
4. Fail any package with a wall above `TEST_TIMEOUT`, as today.
5. When fewer than 3 reference packages remain, report only. Do not fail.
6. Lower `C(p)` only on `main`, as the current tighten step does.
7. Raise `C(p)` only through a change to the baseline file in the same PR. A PR that adds real work to p records the new `C(p)` with a proposed re-baseline verb. The reviewers see the raise in the diff. A raise without a baseline change fails by design.

Step 7 is necessary. Today `tighten` lowers only `p90` (`go/cmd/testlatency/ratchet.go:92`), and `p90` drives only the warning (`ratchet.go:72`). The hard limit is the fixed `2 × class ceiling` (`ratchet.go:50-55`). This judgment moves the hard fail onto `C(p)`. A one-way `C(p)` then fails each PR that adds real work: F3.1 and F4.9 show code changes that move N by 22% to 38%.

The reasons for the numbers:

- The largest clean increase of N was 1.32× (ubuntu `internal/testmainexit`, 6 s, under the floor). For a package with more than 15 s of baseline CPU, the largest clean increase was 1.26× (ubuntu `test/structure` and `internal/core`).
- The smallest defect increase was 1.48× (macOS bridge).
- The 15 s floor stops the noise of small packages, as the current `warnMargin` of 30 s does.

What changes this decision: a clean run on a new host class with N above 1.35× for a large package. Then record that class as its own series (R2). Do not raise the ratio first.

### 6.3 What cycle 2360 implemented

- `testlatency -ratchet <file> -rusage <ndjson>` reads the package CPU of each test binary. A missing `-rusage` gives exit 2.
- The ratchet file has an optional `cpu` map for each package, keyed by the OS name. A baseline of another OS never gates the host.
- The command computes S as the median ratio of the other reference packages. A package fails when N is above 1.35 times its baseline and 15 s over it.
- A package with fewer than 3 other reference packages is reported only. The wall of a package must stay under the `go test` timeout.
- `testlatency tighten -rusage <ndjson>` lowers a baseline, or seeds a missing one. It never raises a baseline and never adds a package.
- Both ratchet steps of `.github/workflows/go.yml` pass `-rusage`. Neither step passes `-ratchet-report-only`, and the flag is gone from the tool.
- A PR that adds real work to a package raises its baseline in the same PR. The reviewers see the raise in the diff of `go/.test-latency.json`. A raise without that change fails by design.

The committed file has no `cpu` value yet, because the artifacts of the runs were not available to the build. Thus the gate judges nothing until R10 seeds the baselines. Until then, the wall check against the `go test` timeout is the only hard fail.

## 7. Limits

- Five runs on each OS is a small sample. The ratio 1.35 is 0.09 above the largest clean increase of a large package (1.26) and 0.13 below the smallest defect increase (1.48). A small package reached 1.32, 0.03 below the ratio. The 15 s floor also stops it.
- The macOS darwin set has only 4 reference packages with 2 s or more of CPU. S on macOS rests on 3 packages after the leave-one-out. A frozen calibration package can make S more robust.
- S is the median of the references. A regression in more than half of the reference packages at the same time moves S and hides in N. A change to a shared helper or to the race detector can do this. The raw CPU trend in the report must show such a change.
- `wait4` adds the CPU of the child processes that the test binary reaps. N therefore counts git, `go build` and `log show`. N does not count a wait on a real clock. The 60 s fence deadline of F4.8 adds wall but almost no CPU, so only the wall report and R4 cover it.
- The class of each macOS host is an inference from the concurrency and the memory. The logs do not print the core count (R2).
