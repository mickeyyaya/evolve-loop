# 2026-10-06 — bridge tests went red whenever another floor ran: freshly written fake CLIs waited on XProtect, on a shared, stale tmux server, and the commit gate's lint refused to share its lock

**Class:** test infrastructure (false reds in the local floor), plus two production defects found while investigating:
- tmux matches `-t` session names by prefix;
- the commit gate's golangci-lint refuses to run beside another golangci-lint.

**Surface:**
- every test in the module that wrote a fake CLI and ran it directly;
- `go/internal/bridge` real-tmux tests;
- `bridge.execTmux`, `swarm.ExecTmuxKill` and the observer's pane probe;
- the orphan-socket GC;
- `internal/commitgate`'s Go lane.

**Inbox:** `realtmux-tests-interfere-across-concurrent-test-processes` (2026-10-05) and `realtmux-bridge-tests-flake-under-package-parallel-load` (2026-09-29).

## What happened

From 2026-09-30, the `TestRealTmux_*` group failed in console floors, mostly inside `apicover-enforce`, which re-runs `internal/bridge` with `-tags integration -coverprofile`.

- It failed with `REPL prompt never appeared after 60s` (rc 80) or an artifact timeout (rc 81), in about 8 s, not 60 s.
- Every red overlapped another worktree's floor or a console lane running bridge tests.
- Every isolated rerun passed, even at a load average of 45.

The concurrent floors run for this investigation showed the same pattern in more tests:

| Test | Deadline | When it went red |
|---|---|---|
| `TestLaunchProfilePolicyWithFixtureChild` | 10–20 s | every concurrent floor |
| `TestNativeRetrospectiveLessonBoundary` | 10–20 s | every concurrent floor |
| `TestNativeRoleEvalAuthoringBoundary` | 10–20 s | every concurrent floor |
| `TestNativeDecisionProfileOwnedCWD` | 10–20 s | every concurrent floor |
| `internal/commitgate`'s `TestGolden_GoPipelineWritesByteExactAttestation` | — | whenever another golangci-lint ran |

## Investigation

### Reproduction

Two apicover-style floors ran at once over the 237 enforced packages, with a logging `tmux` shim first on `PATH`.

- Both floors went red in `internal/bridge` alone.
- Two to four concurrent `-run TestRealTmux` binaries, or three concurrent full bridge binaries, all passed. They link no new binaries while they run.
- The shim log put each session's time from launch to prompt delivery at 3.4–8.3 s in the concurrent floors, against about 0.5–1 s isolated.
- The fixtures scale each 1 s boot poll to 120 ms, so the 60-poll budget is about 8.3 s of wall time.
- tmux itself stayed responsive throughout: the polls of a failing session ran about 145 ms apart.

### Cause 1, the dominant one: XProtect scans every freshly written executable on its first run

**The pattern.** Many tests write a fake CLI (a bash or sh script, mode 0755 or 0700) and then run it directly:
- as the launch command in a tmux pane;
- as `BRIDGE_<CLI>_BINARY`;
- through `PATH`;
- as a git hook.

**The scan.** macOS assesses a freshly written executable on its first run (`XprotectService`), and scripts are no exception. A concurrent floor builds and runs hundreds of freshly linked test binaries, which keeps `XprotectService` at about 50–85% CPU, and every first run waits in its queue.

**Measured on the host.** The "fresh executable load" column used a generator of 24 workers that write and run fresh scripts, which reproduces the scan delay on demand.

| First run of | Another floor running | Fresh executable load |
|---|---|---|
| a freshly written script | 10–15 s | 4–22 s |
| a fresh copy, or APFS clone, of an already-scanned binary | — | 6–22 s (also a new file) |
| a hard link or symlink to an already-scanned binary | — | 0.01–0.05 s |
| `bash <fresh script>` (the new file is only read) | 0.01 s | 0.01–0.03 s |
| a fresh copy of an Apple-signed binary | 0.016 s | — |
| a freshly written script, quiet host | 0.1–0.3 s | — |

**Inside the bridge suite.** In the same failing runs, the python and node fixtures, which launch an installed interpreter with the script as an argument, booted in 0.16–0.6 s. Every directly run bash fake took 7–12 s or never appeared.

**Why concurrency and not load.** The delay comes from other processes creating new executables, not from CPU load, which is why isolated reruns at load 45 passed.

### Cause 2, smaller: every test process shared one stale tmux server with login-shell panes

Every bridge test process used the default `tmux -L evolve-bridge` server, because only the loop sets `EVOLVE_TMUX_SOCKET`.

- That server (pid 20943) had been started on 2026-09-30 22:11:55 by `TestRealTmux_ConcurrentSessionsIsolated`, in pid 3948, in another worktree.
- The process died and left three sessions behind, which kept the server alive for six days.
- Every later test pane ran with the dead process's environment (a deleted `GOCOVERDIR`, its working directory, a shared Apple Terminal session id, its `PATH`).
- Every pane started the operator's login `zsh`, whose `~/.zprofile` runs `pyenv init` twice. Each run takes pyenv's host-wide rehash lock: 1, 8 and 16 login shells at once took 0.33, 1.91 and 3.72 s.
- A pane became ready in about 0.5 s on a quiet host and up to 3.2 s under concurrency. A `/bin/sh` pane took at most 0.36 s under the same concurrency.

### Cause 3: a fixed 8 s sender in `TestRealTmux_E2E_LiveInjection_UnblocksAgent`

The test's injection sender re-appended its command for a fixed 8 s, independent of the boot. `einject` booted in 8.16 s and 8.30 s in the concurrent floors, so the sender had already stopped when the prompt was delivered, and the run ended rc 81.

### The commit gate's golangci-lint lock

`golangci-lint run` takes a host-wide lock (`golangci-lint.lock` in the temp directory). Without a runner option, it exits with `parallel golangci-lint is running` whenever another run holds the lock.

- The commit gate runs `golangci-lint run` in its Go lane. It is the only production caller; the build floor does not lint.
- So a console commit red whenever the operator, another console lane or a test linted at the same moment.
- `TestGolden_GoPipelineWritesByteExactAttestation` drives the real gate, so it went red in every floor run beside another.
- A live check with a run holding the lock: plain `run` exited 3; `run --allow-serial-runners` waited about 7 s and passed.

### No killer

The shared server had never restarted since 2026-09-30, so no `kill-server` ran on the default socket during the reds. The production reapers were already safe:
- `ReapOrphanSessions` needs a `-pid<N>` token;
- `ReapOrphanSockets` matched only `evolve-bridge-p<pid>`;
- `ReapRunSessions` and `sessionreaper` act only on registries;
- `runSocketTeardown` kills only the loop's own socket.

No fixed temporary path was shared by the bridge tests. When two floors ran in one worktree, `internal/phasespec`'s real-tree decoy collided; the dev/cl-decoy lane owns that one.

### A real cross-process kill path

tmux resolves a `-t` name that matches no session to the single session whose name starts with it. On a private socket:
- `kill-session -t evolve-bridge-it-happy-3948` killed a live `evolve-bridge-it-happy-39481`;
- `has-session` reported it present;
- `display-message` printed the other session's values with exit 0.

Where this could fire:
- the bridge's second cleanup kill;
- registry reapers, which kill sessions that have already ended;
- named swarm sessions such as `-w1` and `-w10`.

So a reaper or a test could kill, type into or read another process's session. Separately, `TestTmuxKiller_SatisfiedByExecTmuxKill` ran a real kill on the operator's shared socket.

## Fix

- **No test writes a fresh executable for code to run.** The new `internal/fakeclitest` is the one helper for the class.
  - `fakeclitest.Install(t, path, body)` writes the script beside `path` (not executable) and hard-links the running, already-scanned test binary at `path`.
  - The package's `init` turns that binary into the script's interpreter (`#!` line, else `/bin/sh`) whenever a script sits beside it.
  - Every directly run fake in the module now goes through it (the list is below).
- **Exact targets.** `bridge.ExactSessionTarget` (`=<name>:`) is the one spelling of a tmux target. Every `execTmux` method, `swarm.ExecTmuxKill` and the observer's capture use it. The swarm test is stubbed.
- **One tmux server per test process.**
  - `internal/tmuxtest.Main` is the `TestMain` of `internal/bridge` and `internal/looppreflight`.
  - It selects `evolve-bridge-t<pid>` and starts that server itself, with no user config, `exit-empty off` and `/bin/sh` panes. It kills only that server at exit.
  - The orphan-socket GC reaps a `-t<pid>` socket only when its owner pid is dead.
- **The einject sender lives as long as the run.** It re-appends until the artifact appears or `runTmuxREPL` returns, so the run's own boot and artifact budgets bound it.
- **The commit gate waits for golangci-lint's lock, within a bound.**
  - It runs `golangci-lint run --allow-serial-runners`.
  - Serial rather than parallel keeps runs from writing the shared analysis cache at once.
  - golangci-lint 2.13.2 on the host supports the option.
  - The wait is bounded. The architecture review found it still waiting at 45 s behind a suspended run, and golangci-lint's `--timeout` does not cover the lock wait. So the lint runs under `Options.LintBudget` (default `defaultLintBudget`, 10 minutes), and on the deadline the lane fails, logging `golangci-lint waited <budget> for another run's lock or its own lint, and was stopped`.
- **Test-only helpers stay out of production code.**
  - `fakeclitest` follows the `*test` naming of the other test-only helpers.
  - `internal/repocontract`'s `TestTestOnlyHelpersAreNeverImportedByProductionCode` fails on any non-test import of `internal/fakeclitest` or `internal/tmuxtest`. It runs at every ship.
- **A symlink to a fake runs its script.** `runIfFake` resolves symlinks before looking for the script, because darwin's `os.Executable` returns the path the process was started by.

### What moved to `fakeclitest.Install`, and what was left alone

**Converted.** Every test that writes a fake and has the code under test run it directly:

| Package | Fakes |
|---|---|
| `internal/bridge` | the three real-tmux fake REPL writers, and the four `BRIDGE_<CLI>_BINARY` stubs (10–20 s deadlines) |
| `cmd/evolve` | the tmux shims (bounded tmux calls, 15 s reap), the gh shim (60 s wave sync), the pre-commit hook, the e2e fake ship |
| `internal/core` | the git shim |
| `internal/dossier` | the pre-commit hook |
| `internal/posteditvalidate` | the python stub |
| `internal/releasepreflight` | the gh and go shims |
| `internal/rollback` | the 15 gh, git and evolve fakes |
| `internal/consensusdispatch` | `writeExec` (fake evolve) |
| `internal/ciparity` | the `probe-go` wrapper (2 min) |
| `internal/tmuxtest` | the fake tmux |

Several of the converted fakes have no deadline of their own. They were converted anyway: each first run can add 10–20 s to its package under a concurrent floor, and a package's total stays bounded by the `go test` timeout.

**Left alone, with the reason:**

| Never run, or run another way | Reason |
|---|---|
| placeholder "binaries" in `cmd/evolve` (`cmd_loop_boot_*`, `cmd_loop_chain_*`, `cmd_loop_reexec_handoff`), `internal/core` (`buildleak_recover`, `post_build_repin`), `internal/loopchain`, `internal/phaseintegrity` | content is hashed or compared, never run |
| `internal/doctor` probe, `internal/releasepipeline` `resolve_evolve_bin`, `internal/rollback` `ResolveEvolveBin*`, `internal/consensusdispatch` resolution tests | path resolution or exec-bit checks only |
| `internal/posteditvalidate` `ok.sh` / `bad.sh` | read by `bash -n` |
| `internal/marketplacepoll` | asserts the script is not run |
| `internal/cyclesimulator` plugin scripts, the `internal/fanoutdispatch` helper | run as `bash <script>`, so the file is only read |
| `internal/releasepipeline` `anchored_behavior` and `default_release_verify` | the fake's bytes are committed to a fixture repo and hashed, so they are part of the assertion |
| `internal/subagent` parity adapter | the test skips (the legacy bash dispatcher is gone), and the adapter is only checked to exist |
| `acs/cycleN` predicate packages (`cycle1706`, `cycle1745`, `cycle1748`, `cycle1801`) | frozen per-cycle contracts, run only in their own cycle's audit, never in the floor |
| binaries built with `go build` (e2e's `evolve-fake-cli`, binaries built inside tests) | compiled programs, not scripts; scanned once per build, and the e2e tier's budgets are minutes |

## Pinning tests and red-first evidence

**Fresh executables (fakeclitest).**
- **Reproduced red, then green, under fresh executable load:**
  - Before the conversion, 3 of the 4 fixture-child tests failed at 10–20 s (the fourth took 15 s and passed). All 4 passed alone.
  - Five real-tmux tests with directly run fakes failed (8–16 s).
  - After the conversion, all of them passed in 0.03–3 s under the same load, while a fresh script took 14 s to start.
- **`internal/fakeclitest`:**
  - `TestInstall_TheFakeIsTheRunningTestBinaryNotANewExecutable` checks `os.SameFile`. It is the deterministic stand-in, because the scan itself cannot be produced inside a unit test.
  - Also pinned by `TestInstall_RunsTheScriptUnderItsShebangWithTheArgumentsStdinAndExitCode`, `TestInstall_FindsAFakeOnPathByName`, `TestInstall_ReinstallingReplacesTheScript`, `TestInstall_AWriteThroughTheFakeIsRefused`, `TestRunIfFake_ExecsTheInterpreterOnlyWhenAScriptSitsBesideTheExecutable`, `TestInstall_FallsBackToACopyWhenTheBinaryCannotBeLinked` and `TestInstall_ReportsWhatItCouldNotDo`.

**The einject sender.** With a fake that boots in 9 s inside a 13 s boot budget, the fixed 8 s sender ended rc 81. The run-scoped sender passes.

**Review round (architecture review: 0 critical, 4 warnings, 1 nit), each red first:**

| Finding | Fix | Red first |
|---|---|---|
| Lint wait unbounded | bounded by `LintBudget` | `TestRun_GolangciLintRunsUnderABoundedContext` was red: the lint context had no deadline. `TestRun_AGolangciLintThatOutwaitsItsBudgetFailsTheLaneLoudly` and `TestRun_TheLintBudgetDefaultsWhenUnset` would not compile without the budget. |
| Symlinked fakes | `runIfFake` resolves symlinks | `TestInstall_ASymlinkToAFakeRunsTheScript` was red: the symlink ran the test binary (`testing: warning: no tests to run`). |
| Production imports of test-only helpers | the import guard | The guard was red with a probe production file importing `fakeclitest`, and green once it was removed. `TestProductionImporters_FlagsOnlyNonTestFiles` pins the detector. |
| Surviving mutant | a window-width assertion in `TestRealTmux_AMissingSessionNeverResolvesToALongerName` | A mutant leaving `JiggleWindow`'s first resize non-exact passed the old test and fails the new one (`resized … to "79"`). |
| Hard-coded socket names (nit) | the socket-GC tests build their names from `bridge.DeriveTestSocket` and `bridge.DeriveRunSocket` | — |

**golangci-lint lock.**
- `TestRun_GolangciLintWaitsForAnotherRunInsteadOfFailing`. Red: the argv was `golangci-lint run ./.`.
- The live lock check above.

**Exact targets.**
- **`TestRealTmux_AMissingSessionNeverResolvesToALongerName`** (bridge, real tmux).
  - Red: has-session, both display-message reads, send-keys, capture-pane with and without history, resize-window and paste-buffer all reached `<name>9`, and the kill killed it.
- **`TestExecTmuxKill_NeverKillsASessionWhoseNameExtendsTheTarget`** (swarm, real tmux on a private `TMUX_TMPDIR`).
- **`TestTmuxPaneProbe_CapturesOnlyTheExactListedSession`** (observer).

**Isolation.**
- **`TestRealTmux_ConcurrentTestProcessesNeverShareATmuxServer`** (bridge). Red without the `TestMain`: both children reported one server.
- Also pinned by `TestReapOrphanSockets_ReapsADeadTestProcessSocketAndSparesALiveOne`, `TestExecListBridgeSockets_ListsTestProcessSockets`, `TestDeriveTestSocket_IsPerProcessAndNeverTheSharedOrALoopSocket` and the three `TestMain_*` tests in `internal/tmuxtest`.

## Left open

- **Binaries tests build with `go build`.** They still pay one first-run scan per build under a concurrent floor. Their budgets have been large enough so far.
- **`internal/core`'s own run time.** It takes 330–565 s per run on the development host, close to `go test`'s default 10 m package timeout. That is unrelated to this change, but it is a budget to watch.
