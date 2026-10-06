# internal/fakeclitest

## Purpose

`internal/fakeclitest` stands in for a command-line tool in tests (a fake `claude`, `codex`, `gh`, `git`, `tmux`, `go`, a git hook, a fake REPL) without writing a new executable file.

A test calls `fakeclitest.Install(t, path, body)` where it used to write an executable script. Code under test then runs `path` exactly as it would run the real tool: by absolute path, through `PATH` lookup, as a git hook, as the `BRIDGE_<CLI>_BINARY` override, or typed into a tmux pane.

## Design

**Why no new executable.** macOS scans a freshly written executable the first time it runs (`XprotectService`), scripts included. While a floor links and runs hundreds of new test binaries, that scan queues.

Measured on the development host on 2026-10-06, with a 24-worker generator of fresh executables:

| Executable | First-run latency |
|---|---|
| Freshly written script | 10–22 s |
| Fresh copy of an already-scanned binary | 6–22 s (also a new file) |
| APFS clone | 9 s |
| Hard link to an already-scanned binary (what `Install` makes) | 0.01–0.05 s |
| Symlink to an already-scanned binary | 0.03–0.04 s, but see below |
| Script read by an installed interpreter (`/bin/sh <script>`) | 0.01 s |

Tests that wrote their fake CLI and then ran it under a 10–20 s deadline went red whenever another floor ran on the host, and passed alone.

**How `Install` works.**
1. It writes `body` beside `path` as `<path>.fakecli` (mode 0600, never executable).
2. It hard-links the running test binary at `path`. The test binary was scanned when `go test` started it, and a hard link is the same file, so `path` runs at once.
3. It sets the shared file's mode to 0555, so a stray write through the fake fails instead of overwriting the running test binary.
4. If the link cannot be made (another volume), it copies the binary instead and logs that on the test (`t.Logf`). That is correct, but the copy is a new file and is scanned.

**What runs when the fake starts.** The package's `init` runs in every test binary that imports `fakeclitest`, before `TestMain` and before any test.
1. It looks for `<os.Executable()>.fakecli`.
2. A normal test run finds none and carries on.
3. A fake finds its script and replaces itself (`syscall.Exec`) with the script's interpreter: the script's `#!` line, `/usr/bin/env bash` included, else `/bin/sh`. The interpreter gets the script path, the fake's arguments, its standard streams and its environment.
4. The test binary never runs a test in that process. It also never runs Go exit hooks, so a coverage-instrumented binary writes no coverage data from a fake.
5. A script that exists but cannot be read (a permission or sandbox denial, an I/O error) ends the process with exit 127 and a `fakeclitest: read …` line. The alternative would be falling through and silently running the whole test binary in the fake's place.
6. In the script, `$0` is `<path>.fakecli`, not `path`, because the interpreter receives the script file. No fake in the module reads `$0`; a fake that needs its own name must hard-code it.

**Why `init` and not a `TestMain` hook.** A hook would have to be wired into every package that installs a fake, and a package that forgot the hook would silently run its whole test suite as the "fake". With `init`, importing the package is the wiring.

**Symlinks and copies of a fake.** On darwin, `os.Executable` returns the path a process was started by, so a symlink to an installed fake used to miss the script beside its target and run the whole test binary. `runIfFake` resolves symlinks first, so a symlink to a fake now runs its script (`TestInstall_ASymlinkToAFakeRunsTheScript`). A *copy* of a fake is a new file: it is scanned on its first run, and it has no script beside it, so it runs the test binary itself. Never copy a fake; install another one.

**Only test code may import the package.** Its `init` runs whatever script sits beside the binary, so in a production binary it would be a trust-boundary hole. `internal/repocontract`'s `TestTestOnlyHelpersAreNeverImportedByProductionCode` fails on any non-test file that imports `internal/fakeclitest` or `internal/tmuxtest`; it runs in ship's repo-contract pack. The `test` suffix follows the other test-only helpers (`gittest`, `cliroutetest`, `routingtest`, `tmuxtest`).

**Not covered: binaries tests build.** `go build` output (e2e's `evolve-fake-cli`, binaries built inside a test) is a real compiled program, not a script. It cannot go through an interpreter, and it is scanned once per build.

## Invariants

- **A fake is the running test binary, not a new file.**
  - Pinned by `TestInstall_TheFakeIsTheRunningTestBinaryNotANewExecutable` (`os.SameFile` against `os.Executable()`).
  - The scan itself cannot be produced inside a unit test, so this identity check is the deterministic stand-in. The latency evidence is in the incident record.
- **A fake runs its script with the script's interpreter, arguments, stdin and exit code.**
  - Pinned by `TestInstall_RunsTheScriptUnderItsShebangWithTheArgumentsStdinAndExitCode`, which covers `#!/bin/sh`, `#!/usr/bin/env bash` and a script with no `#!` line.
  - Also pinned by `TestInstall_FindsAFakeOnPathByName` and `TestInterpreterArgv`.
- **Only a binary with a script beside it becomes a fake; an unreadable script and a failed exec both exit 127 loudly.** Pinned by `TestRunIfFake_ExecsTheInterpreterOnlyWhenAScriptSitsBesideTheExecutable`.
- **Reinstalling replaces the script; writing through the fake is refused.**
  - Pinned by `TestInstall_ReinstallingReplacesTheScript` and `TestInstall_AWriteThroughTheFakeIsRefused`.
  - A test must never write to or chmod a fake path except through `Install`: both share the test binary's inode.
- **The copy fallback and every failure are reported.** Pinned by `TestInstall_FallsBackToACopyWhenTheBinaryCannotBeLinked`, `TestInstall_ReportsWhatItCouldNotDo` and `TestCopyExecutable_CopiesTheBytesAndRefusesToOverwrite`.

## Findings

- **The fresh-executable first-run scan** (2026-10-06, [incident](../../incidents/2026-10-06-realtmux-tests-share-a-stale-tmux-server.md)).
  - **What went red.** It caused the real-tmux bridge reds: the fake REPL booted past the scaled 8 s budget. It also caused the reds of `TestLaunchProfilePolicyWithFixtureChild`, `TestNativeRetrospectiveLessonBoundary`, `TestNativeRoleEvalAuthoringBoundary` and `TestNativeDecisionProfileOwnedCWD` in every concurrent floor.
  - **Reproduced on demand.** With a generator of fresh executables running, three of the four fixture-child tests failed at 10–20 s. Five real-tmux tests with directly executed fakes failed. After moving to `Install`, all of them passed in 0.03–3 s under the same load.
  - **Other tests moved to `Install` in the same change.** Every other test in the module that wrote a fake and ran it directly:
    - `cmd/evolve`: tmux shims, gh shim, pre-commit hook, e2e fake ship;
    - `internal/core`: git shim;
    - `internal/dossier`: pre-commit hook;
    - `internal/posteditvalidate`: python stub;
    - `internal/releasepreflight`: gh and go shims;
    - `internal/rollback`: gh, git and evolve fakes;
    - `internal/consensusdispatch`: fake evolve;
    - `internal/ciparity`: the `probe-go` wrapper;
    - `internal/tmuxtest`: fake tmux.
  - **Left alone.** The incident record lists every test left as it was, with the reason.
