# Research: the flaky tests of the CI, 2026-10-12 to 2026-10-26

- **Status:** complete, 2026-10-26.
- **Related:** [ci-wall-time-2026-10](ci-wall-time-2026-10.md), [ci-telemetry-and-critical-path](../architecture/ci-telemetry-and-critical-path.md), [hermetic-driver-tests](../conventions/hermetic-driver-tests.md), [internal-tmuxtest](../architecture/packages/internal-tmuxtest.md), [internal-gittest](../architecture/packages/internal-gittest.md), [internal-rawgitratchet](../architecture/packages/internal-rawgitratchet.md), [internal-paralleltest](../architecture/packages/internal-paralleltest.md).
- **Method:**
  - `gh run list --workflow required.yml --created ">=2026-10-12" --limit 1500`: 914 runs. The workflows `ci.yml` and `go.yml` run only as jobs of `required.yml` in this window. Their own lists end on 2026-09-28.
  - `gh api .../runs/<id>/jobs?filter=all` for each of the 914 runs, and `gh run view <id> --attempt <n> --log-failed` for each of the 161 red attempts.
  - The `go test -json` streams (`test-integration.json`) of 473 coverage artifacts of the red runs. The streams exist from C0 on (2026-10-17).
  - `evolve ci classify <run> --json` on three runs, with `--project-root` set to a worktree, so that no item reached the live inbox.
  - Code reads at `origin/main` 6ded88874 with `path:line`, and local repeats of two tests. A quoted CI message keeps the line number of the head of its run.
  - The open and consumed inbox items that name a flake, the console review notes, and `docs/incidents/`.
  - Live web searches on 2026-10-26 (section 8).
- Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

- [1. The request](#1-the-request)
- [2. How the census tells a flake from a fixture](#2-how-the-census-tells-a-flake-from-a-fixture)
- [3. The ranked flakes](#3-the-ranked-flakes)
- [4. The evidence for each flake](#4-the-evidence-for-each-flake)
- [5. Reds that are not flakes](#5-reds-that-are-not-flakes)
- [6. The classes](#6-the-classes)
- [7. Test-design rules and their guards](#7-test-design-rules-and-their-guards)
- [8. Current practice: adopt or reject](#8-current-practice-adopt-or-reject)
- [9. Gaps in the tools](#9-gaps-in-the-tools)
- [10. Limits](#10-limits)
- [11. Inbox items](#11-inbox-items)

## Terms used throughout

- **Flake:** a test that fails and passes on the same code. The census accepts three proofs:
  - a rerun of the same run passes;
  - the same head passes in another run;
  - the test fails on a change that does not touch its package, and `main` passes near that time.
- **Occurrence:** one red job of one run that the flake caused.
- **Fixture text:** output that a passing test prints on purpose, for example a transcript of a refused build floor. It contains `FAIL` lines, but no test failed.
- **Job:** one `build + test` job of the `go` workflow on one platform: macOS Go 1.23, ubuntu Go 1.23 or ubuntu Go 1.27.
- **Retry:** the run that a red job forces: a rerun of the failed jobs, or a new push.

## 1. The request

The operator asked on 2026-10-26: "Please investigate flaky tests which wastes our effort and blocking the pipeline, consider to improve the test itself".

The answer in short:

- Eight tests flaked in 14 days, in 21 occurrences. They cost about 1,135 runner minutes of 34,050 Go job minutes (3.3%).
- Two of the eight are already fixed on `main` (#1006 and #1134). Six are open, and each has one inbox item.
- They made `main` red seven times and held 14 pull requests for a retry.
- Each flake has a cause in the test design, and a rule of this repository can prevent each cause. Section 7 gives the rules and the guards.
- The request named seven candidates. Two are flakes (ranks 4 and 6). Four are real defects: `acs/cycle10` on `main`, the predicates of cycle 1946, the predicates of cycle 1950, and the #1132 hang. The #1132 hang class is also a test-design fault. The `internal/guards` failure has no record. Section 5 gives the detail.

## 2. How the census tells a flake from a fixture

### F2.1 Most FAIL lines in a red log are fixture text

The failed log of a red run contains many `FAIL` lines from passing tests. For example, the log of run 40269836920 `attempt` 1 has 13 `FAIL github.com/.../acs/cycle10`, `cycle1946` and `cycle1950` lines. They are inside `[build-floor] REJECT` transcripts, and they come from the constants at `go/cmd/evolve/cmd_handoff_test.go:31-42`. The only real failure of that job is `TestRealTmux_AKilledSessionLeavesNoEnvFile`.

*Implication:* a count of `--- FAIL:` lines in a log is wrong. The census needs a source that knows which package failed.

### F2.2 The test stream also records fixture text as failures

`go test -json` runs `test2json` on the output of the test binary. A passing test that prints a `--- FAIL: <name>` line at column 0 makes `test2json` record a `fail` event. In the 473 streams, `TestInnerRunCycleErrorF4` has 330 such `fail` events, but its package `internal/routingtest` never failed.

*Implication:* a `fail` event of a test counts only if the same stream has a `fail` event for its package.

### F2.3 The rule that the census used

1. Take the `fail` events of top-level tests from `test-integration.json`.
2. Keep an event only if the package of the same stream also has a `fail` event.
3. For the runs before C0 and the reruns, read the `--- FAIL:` line and its message in the log. Keep it only if a `FAIL <package> <time>` line of a real package follows it. The message must name a file of that package.
4. Read the message of each kept failure, and compare the platforms and the head of the run.

The result: 1,026 confirmed failures of top-level tests in the streams of 74 red runs. The logs of the other red runs show reds of gate targets (`cover-changed`, `apicover-check`) and eight more test reds. Almost all are deterministic: they fail on all three platforms of one branch, and the next push of the branch fixes them. The flakes are the rest.

## 3. The ranked flakes

The rate is the count divided by the jobs on that platform while the test existed. A cost is the minutes of the red job plus the minutes of its retry. A measured rerun gives its own minutes. A retry without a record takes 40.7 minutes, the sum of the mean times of the three jobs. This number is an estimate.

| Rank | Test | Package | Platform | Count, rate | Cost (runner min) | Class | Status |
|---|---|---|---|---|---|---|---|
| 1 | `TestChannel_EndToEnd` | `internal/bridge/channel` | ubuntu 4, macOS 1 | 5 of 1,544 jobs, 0.32% | 271 | poll order across two files | fixed by #1006: 0 of 971 jobs after it |
| 2 | `TestRealTmux_AFailedStartLeavesNoEnvFile` | `internal/bridge` | macOS | 4 of 41 jobs, 9.8% | 210 | async teardown of a pane | fixed by #1134: 0 of 35 jobs after it |
| 3 | `TestCancelKillsTheProcessGroup_CancelReapsTheGrandchildToo` | `internal/phases/ship` | macOS | 3 of 830 jobs, 0.36% | 175 | process and signal timing under load | open, cause not proven |
| 4 | `TestDispatchEvaluateBatch_Concurrent` | `internal/core` | macOS | 3 of 830 jobs, 0.36% | 167 | a sleep that must create an overlap | open |
| 5 | `TestE2ECLIFallbackChain` (cleanup) | `cmd/evolve` | macOS | 2 of 830 jobs, 0.24% | 113 | git maintenance child against `t.TempDir()` | open |
| 6 | `TestRealTmux_AKilledSessionLeavesNoEnvFile` | `internal/bridge` | ubuntu | 2 of 81 jobs, 2.5% | 98 | tmux server lifecycle | open |
| 7 | `TestRealTmux_NewSessionInBindsPaneCwd` | `internal/bridge` | ubuntu 1.23 | 1 of 843 jobs, 0.12% | 52 | one read of an async state | open |
| 8 | `TestWatch_AnOversizedBodyIsRecordedWithATruncationNote` | `internal/practicewatch` | ubuntu 1.27 | 1 of 842 jobs, 0.12% | 49 | a wall-clock deadline under load | open |

The run ids of each row are in section 4. Two costs are upper bounds. Run 39041577057 (rank 1) and run 39262811729 (rank 3) also had other red tests, so a retry was necessary for those reasons too.

Rank 6 has the highest rate of the open flakes. It entered `main` on 2026-10-25 and failed twice in its first day. The ubuntu jobs average 120 for each day (1,685 in 14 days). At 2.5% and 49 minutes for each occurrence, rank 6 costs about 147 minutes for each day. So it becomes the costliest flake in one to two days (synthesis, an estimate).

The #1132 hang is not in the table, because its first cause is a defect of that pull request (section 5, F5.3). It cost the most runner time of any single red: 280 macOS minutes.

## 4. The evidence for each flake

### F4.1 `TestChannel_EndToEnd`: fixed by #1006

- Runs: 38241358938 `attempt` 1 (#874, rerun passed), 39041577057 (`main`), 39052164266 `attempt` 1 (#951, rerun passed), 39055297537 (`main`), 39343069315 (#999, which does not touch `internal/bridge/channel`).
- Message: `e2e_test.go:76: answer text = "", want substring "3 bullets"` (line 112 at an older head).
- Cause: the writer goroutine appends the text to `build-stdout.log` and then the marker `idle_reached` to `build-stderr.log` (`go/internal/bridge/channel/e2e_test.go:67-68`). Before #1006, `Normalizer.classifyNewLines` (called by `Poll`) read stdout and then stderr. If both lines arrived between the two reads, the classifier saw `idle_reached` first. The span of the answer then closed before the text got its sequence number.
- Fix: #1006 (commit 1541b16bf, merged 2026-10-20T04:20Z) reads stderr first and then stdout, and still classifies the stdout lines first (`go/internal/phasestream/normalizer.go:102-117`). `go/internal/phasestream/normalizer_order_test.go` pins the order. The test failed in 5 of 1,544 jobs before #1006 and in 0 of 971 jobs after it.
- Two items name this test. The consumed item `channel-e2e-timing-flake` (2026-09-14) fixed an earlier symptom. The open item `three-tests-flake-under-full-suite-load` (2026-10-17) still lists it, but #1006 removed that part.

*Implication:* no item. The product had the same window, and #1006 fixed it in the product, not in the test.

### F4.2 `TestRealTmux_AFailedStartLeavesNoEnvFile`: fixed by #1134

- Runs: 40215324014 (#1127), 40217002682 `attempt` 1 (#1126, a rerun passed), 40220925284 (`main`), 40229237393 (#1133). All on macOS.
- Message: `tmux_hermetic_env_integration_test.go:119: a failed start must remove its env file: [.../evolve-cellenv-2971474163]` (line 120 at 6ded88874).
- Cause: the round-2 review of #1126 found that the leftover file was the env file of the first, successful session. Its pane had not run its `rm` yet. In 200 scratch cycles, the failed start left nothing.
- Fix: #1134 (merged 2026-10-25T20:46Z) makes `NewSessionEnv` wait until the pane removes its env directory (`go/internal/bridge/tmux.go:151`, `awaitCellEnvConsumed` at `tmux.go:185`). The test failed in 4 of 41 macOS jobs before #1134 and in 0 of 35 after it.

*Implication:* no item. The product change removed the race, and the absolute check of the test is now correct.

### F4.3 `TestCancelKillsTheProcessGroup_CancelReapsTheGrandchildToo`: cause not proven

- Runs: 39069306819 `attempt` 1 (#928, a rerun passed), 39262811729 (#997), 39392251222 (#1032). All on macOS. Neither #997 nor #1032 touches `internal/phases/ship`.
- Message: `repocontract_cmd_unix_test.go:50: Wait = context canceled, want the shell killed by SIGKILL`.
- Fact: `exec.Cmd.Wait` returns the error of the context only when the process status is a success. So on these runs, `Wait` saw the shell exit with status 0 after `cancel()` (Go `os/exec`, `Cmd.Wait`).
- The test runs with `t.Parallel()` and real processes on the loaded macOS runner. The census did not find why the shell can exit with status 0. The macOS process-group kill timing is one candidate (synthesis, not proven).

*Implication:* the first step is a diagnosis, not a fix. On a failure, the test must log the exit status and the time from `cancel()` to the exit. It must also log the state of the grandchild.

### F4.4 `TestDispatchEvaluateBatch_Concurrent`: a sleep must create the overlap

- Runs: 38846276793 (`main`), 39462372493 (#1056), 40268371666 (#1138, which does not touch `internal/core`). All on macOS.
- Message: `evaluate_batch_dispatch_test.go:100: phases did not overlap (maxActive=1, want ≥2) — they ran serially`.
- Cause: each fake runner counts itself active, sleeps 20 ms and leaves (`go/internal/core/evaluate_batch_dispatch_test.go:37-46`). The test proves the overlap only if the second goroutine starts in those 20 ms. On a loaded runner, the scheduler can start it later.

*Implication:* replace the sleep with a barrier. Each runner signals its arrival and waits until both runners arrive, with a deadline of some seconds. A serial dispatch then fails on the deadline with a clear message, and a concurrent dispatch always passes.

### F4.5 `TestE2ECLIFallbackChain`: the git maintenance child against `t.TempDir()`

- Runs: 40217469193 (#1128), 40230631208 (`main`). Both on macOS.
- Message: `testing.go:1232: TempDir RemoveAll cleanup: unlinkat .../TestE2ECLIFallbackChaintrigger_81_falls_back_to_codex.../001/.git: directory not empty`.
- Cause: the e2e project is a raw git repository (`gitInit` at `go/cmd/evolve/e2e_cycle_cli_matrix_test.go:451`). Its file has one allowance in `go/internal/rawgitratchet/baseline.json`. A detached git maintenance child writes into `.git` while `t.TempDir()` removes it. This is the class that `internal/gittest` fixes with `maintenance.auto=false` and `gc.auto=0` (`go/internal/gittest/fixture.go:28-31`). The consumed item `tempdir-cleanup-vs-git-flake` (2026-09-11) records the same class.

*Implication:* move the e2e fixture to `gittest`, or give the repository the same two config values. The ratchet entry then goes away. Run 39489674181 (#1084) has a different red of the same test on two platforms. That red is a real defect of #1084, not this flake.

### F4.6 `TestRealTmux_AKilledSessionLeavesNoEnvFile`: the tmux server exits between two starts

- Runs: 40268053917 (#1147, a docs-only change), 40269836920 `attempt` 1 (#1139, a docs-only change, a rerun passed). Both on ubuntu.
- Message: `tmux_hermetic_env_integration_test.go:160: start early-18: exit status 1` and `start early-1: exit status 1`.
- Cause (synthesis): the test starts and kills 20 sessions on a private socket (`tmux_hermetic_env_integration_test.go:157-165`). No `tmuxtest.StartServer` starts that server, so it has the default `exit-empty on`. Each kill removes the last session, and the server exits. The next start can reach a server that is still in its exit. Two facts support this cause: the first start (`early-0`) never failed, and a failure needs a server that existed before.
- Local repeats: `-count=30` on darwin passed 30 of 30. 300 start and kill cycles on a private socket with tmux on darwin had no failure. No Linux host was available, so the cause stays a hypothesis.
- A gap: `NewSessionEnv` returns `exit status 1` without the output of tmux (`go/internal/bridge/tmux.go:148-158`). The red log thus cannot show the cause.

*Implication:* start the private server with `tmuxtest.StartServer(socket)`, which sets `exit-empty off`, and stop it in the cleanup. Also add the tmux output to the start error.

### F4.7 `TestRealTmux_NewSessionInBindsPaneCwd`: one read of an async state

- Run: 38919986816 (`main`, ubuntu Go 1.23).
- Message: `tmux_repl_integration_test.go:357: pane_current_path="", want "/tmp/TestRealTmux_NewSessionInBindsPaneCwd.../001"`.
- Cause: the test reads `#{pane_current_path}` once, right after `new-session` (`tmux_repl_integration_test.go:347-357`). On Linux, tmux reads the path from the pane process, and that process can be absent for some milliseconds.

*Implication:* poll the value until a deadline, and fail only when the deadline passes.

### F4.8 `TestWatch_AnOversizedBodyIsRecordedWithATruncationNote`: a deadline of 3 s

- Run: 39209179814 (`main`, ubuntu Go 1.27).
- Message: `hardening_test.go:340: first = {... Status:error Detail:read http://127.0.0.1:38043: context deadline exceeded ...}`.
- Cause: `optionsFor` sets `Timeout: 3 * time.Second` for each test (`go/internal/practicewatch/watch_test.go:90`). This test reads a body of more than 8 MiB from a local server, twice. On a loaded runner under `-race`, the read took more than 3 s.

*Implication:* a test that is not about the deadline must not depend on it. Give the tests of truncation a long deadline, and keep the short deadline only in the test that proves the deadline.

## 5. Reds that are not flakes

### F5.1 `acs/cycle10` `TestC10_001_DeadFlagsAbsentFromRegistry`: a real defect on `main`

- `go test -count=1 -tags acs -run TestC10_001 ./acs/cycle10/` fails on `origin/main` 6ded88874: `flagregistry.Lookup("EVOLVE_WORKTREE_PATH") returned (flag, true)`.
- The row is `go/internal/flagregistry/registry_table.go:29`, and no production file reads the flag. The predicate is correct, and the row is stale.
- The CI runs only `./acs/regression/...` with the tag `acs` (`.github/workflows/ci.yml:66`), so the CI never runs this predicate. A build floor that runs it fails each time. The `acs/cycle10` lines in the CI logs are fixture text (F2.1).

### F5.2 `acs/cycle1946` and `acs/cycle1950`: defects of the predicates of two cycles

- Neither package is on `main`. They existed in the worktrees of cycles 1946 and 1950.
- Cycle 1946 asked for 100% coverage of `writeBuildSelfCheckArtifact`, but the missing branch needs a seam in a file that the phase must not edit. Cycle 1950 had two predicates in conflict: 001 asks for a dead branch to be covered, and 002 forbids the edit that removes it.
- `go/cmd/evolve/cmd_handoff_test.go:31-42` keeps both failures as fixtures. Their `FAIL` lines in the CI logs are fixture text from passing tests. They are not flakes and not CI reds.

### F5.3 The #1132 hang: a defect of the change, made worse by the tests

- Run 40234187614 (#1132): the macOS job of `attempt` 1 ran from 21:11 to 01:08 and the runner cancelled it. The macOS job of `attempt` 2 failed 15 bridge tests on macOS and was cancelled after 43 min. Cost: 280 macOS minutes.
- Three causes stack, as the description of PR #1141 proves:
  - a credential deny of #1132 covers the cell path;
  - 22 integration waits use `context.Background()`;
  - a tmux server that outlives the test binary holds the capture pipe open.
- The first cause is a real defect of #1132. The other two are test-design faults that turn a red into a hang. PR #1141 pins all three, and it is still open on 2026-10-26.

### F5.4 Other reds that looked intermittent

| Red | Why it is real |
|---|---|
| `TestRunCheckpoint_*` and `TestRunGC_ExplicitRunArchives...` on ubuntu, 20 runs, 13 on `main`, 2026-10-17 14:07 to 15:37 | `git commit-tree` failed with "Author identity unknown". The ubuntu runner has no git identity, and the macOS runner has one. A deterministic dependence on the host. |
| `TestDispatchSweep_AnUntaggedBusyLoop...` on #1103 | The change itself compared a `ps` start time in whole seconds with the wall clock (review of #1103, round 2). The fix came before the merge. |
| `TestAppendLedger_InterleavedWithTheFileLedgerTheChainVerifies` on #1035 | The new test of the change failed in 6 of 30 local runs (review of #1035, H1). The tip format of the subagent did not match `FileLedger.Verify`. |
| `TestE2ECLIFallbackChain` on #1084 | A red on two platforms with a logic message. #1084 changes `cmd/evolve-fake-cli`. |
| A one-time `internal/guards` failure in a local push | The census found no log of it. The CI streams show only two deterministic `internal/guards` reds, each on one branch. The census cannot classify it. |

## 6. The classes

| Class | Flakes | How it fails | Rule (section 7) |
|---|---|---|---|
| Async state read once | F4.2 (fixed), F4.7 | the test reads a state that another process sets later | R1 |
| A sleep that must create an order or an overlap | F4.4 | the scheduler does not keep the expected order under load | R2 |
| A wall-clock deadline in a test that is not about time | F4.8 | a loaded runner passes the deadline | R3 |
| Poll order across two sources | F4.1 (fixed) | one poll sees the later event first | R4 |
| A shared server lifecycle | F4.6 | the server of the test exits or changes between two steps | R5 |
| A git child against the temporary directory | F4.5 | a detached child writes while the cleanup removes the directory | R6 |
| Process and signal timing under load | F4.3 | not proven | R7 |
| An unbounded wait | F5.3 | a red becomes a hang of hours | R8 |
| A dependence on the host | F5.4, checkpoint | a value exists on one runner and not on another | R9 |

The classes agree with the published studies: async waits and concurrency are the largest causes of flakes (section 8).

## 7. Test-design rules and their guards

Each rule says what a test must do, what already enforces it, and where a guard can enforce the rest.

| Id | Rule | Findings | Exists today | Where a guard can enforce it |
|---|---|---|---|---|
| R1 | A test that reads a state that another process sets must poll it until a deadline. It must not read it once. | F4.2, F4.7 | No shared helper. `waitForFeed` in `internal/bridge/channel/e2e_test.go:12` and `gone` in the busy-loop test are local copies. | A shared helper `testhost.Eventually` (or one in `internal/tmuxtest` for tmux reads). An AST guard over integration-tagged files can flag a `display-message` or a `Glob` result that a test checks without the helper. |
| R2 | A test of concurrency must prove the overlap with a barrier and a deadline. It must not sleep. | F4.4 | Nothing. 113 `time.Sleep` calls are in 76 test files. | A ratchet of `time.Sleep` calls in tests, like `internal/rawgitratchet`: a list of allowances that can only shrink. |
| R3 | A test that is not about a deadline must not depend on a short wall-clock deadline. Inject the clock or the deadline. | F4.8 | Some packages inject `Now` (`practicewatch` does it for timestamps but not for the fetch deadline). | Review only. `testing/synctest` (Go 1.25) gives a fake clock, but the CI still runs Go 1.23, so it cannot be used yet. |
| R4 | When the product reads two sources, the test must script the order of the reads, not race them. | F4.1 | `go/internal/phasestream/normalizer_order_test.go` (#1006) pins the read order of the normalizer. | Review only. |
| R5 | A test that runs real tmux must run on a private socket. A test that needs its own server must start it with `tmuxtest.StartServer` (`exit-empty off`) and stop it in its cleanup. | F4.6 | `tmuxtest.Main` gives each binary a private socket in `internal/bridge` and `internal/looppreflight`. `cmd/evolve` and `internal/swarm` set `TMUX_TMPDIR`. Each package that runs real tmux has one of the two. | A guard in `internal/bridge` can fail a test that sets `TmuxSocketEnv` to a new socket and does not call `tmuxtest.StartServer` for it. |
| R6 | A git fixture must come from `internal/gittest`. | F4.5 | `internal/rawgitratchet` fails a new raw fixture. 117 files still have allowances in `baseline.json`. | The ratchet exists. Migrate the files that flaked first, starting with `cmd/evolve/e2e_cycle_cli_matrix_test.go`. |
| R7 | A test of process signals must report the exit status and the times on failure, so that the next red names its cause. | F4.3 | Nothing. | Review only. |
| R8 | An integration test must bound each wait with a context deadline of the test. A test binary must not leave a process that holds its output pipe. | F5.3 | PR #1141 (open) adds `integration_wait_guard_test.go` for the bridge, and `tmuxtest.Main` stops an orphan server. | After #1141 merges, extend the AST guard from `internal/bridge` to every integration-tagged package. |
| R9 | A test must not read a value of the host that the CI runners do not share: a git identity, a login shell, the environment. | F5.4 | `hermetic-driver-tests.md` for `Deps.LookupEnv`, `gittest` for the git identity, `tmuxtest` for the pane shell. | `internal/sysexec` already routes processes through `Command`. A guard can fail a test that runs `git commit` outside `gittest`. |
| R10 | A parallel test must not change a package global. | none | `internal/paralleltest` in `cmd/evolve`, `internal/core` and `internal/phases/ship`. | Add the guard file to more packages. No flake of this window came from this class. |
| R11 | A test that prints a transcript of `go test` output must indent it or write it with `t.Log`. A `--- FAIL:` line at column 0 becomes a false `fail` event (F2.2). | F2.1, F2.2 | Nothing. | A guard can scan test sources for string constants that start a line with `--- FAIL:` or `FAIL\t`. |

## 8. Current practice: adopt or reject

| Id | Practice | Source | Decision for this repository |
|---|---|---|---|
| P8.1 | Async waits, concurrency and test order are the most common causes of flakes in the empirical studies (Luo and others, FSE 2014, 201 fixes). Later studies of JavaScript and of SAP HANA agree on concurrency. | [Luo and others](https://people.cs.vt.edu/nm8247/publications/empirical-study-flaky-pdf.pdf), [JavaScript study](https://ar5iv.labs.arxiv.org/html/2207.01047), [SAP HANA, 2026](https://arxiv.org/pdf/2602.03556) | Adopt: R1, R2 and R4 address the two largest classes. |
| P8.2 | Google measured that about 1.5% of test runs were flaky. It quarantines a test with a high rate and files a bug. A retry hides real failures if nobody tracks it. | [Google Testing Blog, 2016: "Flaky Tests at Google and How We Mitigate Them"](https://testing.googleblog.com/2016/05/flaky-tests-at-google-and-how-we.html) | Adopt the tracking (one item per flake, this census). Reject an automatic retry in CI, because it hides the rate that this census measured. |
| P8.3 | `testing/synctest` (Go 1.25) runs a test in a bubble with a fake clock, and `synctest.Wait` waits until each goroutine of the bubble blocks. | [Go Blog: testing time](https://go.dev/blog/testing-time), [pkg.go.dev](https://pkg.go.dev/testing/synctest@go1.26.8) | Defer: the CI runs Go 1.23. It cannot help the tests of real processes, sockets or files. |

## 9. Gaps in the tools

- **F9.1** `evolve ci classify` reports one more failure with an empty package for each red run, labelled `unknown`. So `retry_safe` is `false` for each run without `--rerun`. Seen on runs 40268053917, 40268371666 and 40230631208.
- **F9.2** `recurred_on_main` reads the last 10 runs of `main` (`ciMainWindow` in `go/cmd/evolve/cmd_ci.go:27`). A flake with a rate of 0.4% almost never recurs in 10 runs, so the label stays `unknown`. A record of past flakes by test name, like this census, is the evidence that the classifier lacks.
- **F9.3** `classify` reads only the latest `attempt` of a run. The open item `ci-classify-reads-a-chosen-attempt` covers it.
- **F9.4** Fixture text makes both the log and the stream report false failures (F2.1, F2.2). R11 removes the source.

## 10. Limits

- The window has 14 days and 914 runs. A flake with a rate under 0.1% can be absent from it.
- The census reads only red runs. A run that the concurrency group cancelled hides its result.
- A rerun replaces the stream of `attempt` 1. For those runs, the census read the log only.
- The rate uses all jobs of a platform as the denominator. Some jobs did not run the package of the test, so a real rate can be higher.
- The costs of a retry without a record are estimates (section 3).
- The causes of F4.3 and F4.6 are not proven. No Linux host was available for a local repeat.

## 11. Inbox items

The console filed one `console-manual` item for each open flake, in the order of section 3. F4.1 and F4.2 have no item, because #1006 and #1134 fixed them.

| Rank | Item |
|---|---|
| 3 | `flake-ship-cancel-wait-reports-a-clean-exit` |
| 4 | `flake-evaluate-batch-overlap-by-sleep` |
| 5 | `flake-e2e-fallback-raw-git-tempdir-cleanup` |
| 6 | `flake-realtmux-killed-session-server-exits` |
| 7 | `flake-realtmux-pane-cwd-read-once` |
| 8 | `flake-practicewatch-oversized-body-deadline` |
