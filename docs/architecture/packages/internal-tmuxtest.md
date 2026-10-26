# internal/tmuxtest

## Purpose

`internal/tmuxtest` gives a test binary that runs real tmux a tmux server that only its own process owns. A package adopts it with one line:

```go
func TestMain(m *testing.M) { os.Exit(tmuxtest.Main(m)) }
```

`internal/bridge` (in an external `bridge_test` file, because this package imports `bridge`) and `internal/looppreflight` adopt it. `cmd/evolve` isolates itself differently, by pointing `TMUX_TMPDIR` at a temporary directory in its own `TestMain`, and does not need it.

## Design

`Main` does six things, in order:

1. It sets `EVOLVE_TMUX_SOCKET` (`bridge.TmuxSocketEnv`) to `bridge.DeriveTestSocket(os.Getpid())`, `evolve-bridge-t<pid>`. The new value replaces any inherited value. Every bridge tmux call goes through `bridge.TmuxSocketArgs`. Thus each session of the tests and each child process use that socket. A test run in the pane of a live loop does not get the `evolve-bridge-p<pid>` socket of the loop.
2. It marks each inherited descriptor above 2 close-on-exec (`closeInheritedOnExec`). It reads the list from `/dev/fd` (the `fdDir` seam). A list that it cannot read stops the run with exit 1. Thus no child of the test binary keeps a pipe that the caller of `go test` reads.
3. If tmux is on `PATH`, it stops a server that is already on that socket. A dead process with a recycled pid can leave such a server. Then it starts the server in one tmux call:
   `-f /dev/null start-server`, `set-option -s exit-empty off`, `set-option -g default-shell /bin/sh`, `set-option -g default-command "exec /bin/sh"`.
4. It starts the orphan guard (`startOrphanGuard`), the session `tmuxtest-orphan-guard`. The guard waits with `kill -0 <pid>` each second. About 1 s after the test binary dies, the guard stops each `<socket>-*` server in the socket directory. Then it stops its own server.
   - Thus it also stops the `ownTmuxServer` servers of `internal/bridge`, which tests start on sockets of their own.
   - The guard does not stop `evolve-bridge-t1234` for pid 123, because that socket has no `-` after the pid.
   - A guard that cannot start stops the server, and the run ends with exit 1.
5. It runs the tests.
6. It stops that server with `swarm.ExecKillServer`, which also removes the socket file. It never touches another socket.

Why each server option:

- **The process starts its own server.** A tmux server's global environment is the environment of the client that started it. On the shared default socket, one server lived from 2026-09-30 to 2026-10-06, kept alive by sessions a killed test had leaked. Every later test pane on the host ran with that dead process's `GOCOVERDIR`, working directory, terminal session id and `PATH`.
- **`/bin/sh` is the pane shell.** Without it, every pane started the login shell of the operator. On the development host `~/.zprofile` runs `pyenv init` two times, and each run takes the host-wide rehash lock of pyenv. Concurrent login shells queue on that lock: 1, 8 and 16 shells at once took 0.33, 1.91 and 3.72 s. A pane became ready in about 0.5 s on a quiet host and up to 3.2 s under concurrency.

  A `/bin/sh` pane took about 0.11 s, and at most 0.36 s under the same concurrency. The real-tmux fixtures drive a scaled boot budget of about 8 s, so two floors at once pushed the boot over it.
- **`-f /dev/null`** keeps the operator's `.tmux.conf` out of the tests.
- **`exit-empty off`** keeps the server alive between tests. Otherwise the client of the next test starts a new server, with the user config and the login shell.

`StartServer(socket)` and `StopServer(socket)` are the steps 3 and 6 alone (exported 2026-10-09). A test that must kill a server uses them on a private socket of its own, never on the shared socket of the binary. For example, `TestRealTmux_AKilledServerEndsTheWatchingDispatchAsPaneLostWithinOneLivenessInterval` uses `evolve-bridge-t<pid>-panelost`. It starts that server, kills it during a dispatch and stops it in cleanup ([cycle 1853](../../incidents/cycle-1853-secondary-defects.md)). `StartServer("")` refuses before it starts anything, because an empty name selects the default server. Pinned by `TestStartServer_RefusesAnEmptySocketBeforeItStartsAnything`.

A test binary whose tmux server cannot start fails with exit 1 and runs nothing. It never falls back to the shared server. A test binary on a host without tmux still gets its own socket name, and the real-tmux tests skip as before.

## Invariants

- **One server per test process, named by its pid, and killed only by its owner, its guard or the dead-owner GC.** `swarm.ReapOrphanSockets` reaps `evolve-bridge-t<pid>` only when that pid is dead, the same liveness gate it applies to a loop's `evolve-bridge-p<pid>`. A test binary that its timeout panic stops skips step 6. Its guard then stops the server in about 1 s, and the next `evolve gc` or loop preflight removes the socket file. `TestMain_RunsTheTestsOnAFreshServerOwnedByThisProcess` pins the socket, the pane shell, `exit-empty`, the replaced stale server, the exit code and the removed socket. In `internal/swarm`, `TestReapOrphanSockets_ReapsADeadTestProcessSocketAndSparesALiveOne` pins the GC.
- **It fails loudly, never falls back.** Pinned by `TestMain_AServerThatCannotStartRunsNothingAndFails` and `TestMain_WithoutTmuxStillRunsTheTestsOnTheirOwnSocketName`.
- **The server and the children of a test binary end with it and hold no inherited pipe (#1132).** `TestStartOrphanGuard_TheServersOfTheOwnerStopSoonAfterItDies` stops the owner of a guarded server on a private `TMUX_TMPDIR`. The server must stop within 10 s. `TestCloseInheritedOnExec_AChildThatOutlivesTheTestHoldsNoInheritedPipe` makes a pipe without close-on-exec and starts a child that lives longer. The read end must get an end of file in 5 s. `TestMain_AGuardThatCannotStartStopsTheServerAndRunsNothing` and `TestMain_AnUnreadableDescriptorListRunsNothingAndFails` pin the two failures.
- **Concurrent test processes never share a server.** Pinned end to end by `internal/bridge`'s `TestRealTmux_ConcurrentTestProcessesNeverShareATmuxServer`, which runs two copies of the bridge test binary at once.
- **Test code only.** `internal/repocontract`'s `TestTestOnlyHelpersAreNeverImportedByProductionCode` fails on any non-test file that imports this package, since `Main` starts and kills tmux servers and rewrites `EVOLVE_TMUX_SOCKET`.
- **No defer before exit.** `Main` returns its code, and the adopting `TestMain` calls `os.Exit` on it, so the `testmainexit` analyzer has nothing to flag.

## Findings

- **Real-tmux reds when another process ran bridge tests at the same time** (inbox `realtmux-tests-interfere-across-concurrent-test-processes` and the earlier `realtmux-bridge-tests-flake-under-package-parallel-load`). Two apicover-style floors ran at once, with a logging `tmux` shim on `PATH`. Both went red in `internal/bridge` alone. The boot latency of a session rose from about 0.5–1 s to 3.4–8.3 s. The 8 s injection sender of `TestRealTmux_E2E_LiveInjection_UnblocksAgent` stopped before the prompt arrived. No test or reaper killed the sessions.

  Two causes stacked. The first was the shared, stale, login-shell server above (the part of this package). The second, and the larger, was the first-exec scan of macOS. It scanned the fake CLI scripts that the fixtures wrote and then ran directly, for 10–15 s per first exec while another floor ran. The bridge fixtures now install their fakes with `fakeclitest.Install` ([internal-fakeclitest.md](internal-fakeclitest.md)). The install is a hard link to the scanned test binary, which runs the script through its interpreter.

  Isolation alone was not enough. Before that fixture change, a verification floor beside two more floors still timed out each fake that it ran directly. The full record is the [incident](../../incidents/2026-10-06-realtmux-tests-share-a-stale-tmux-server.md).
- **A timeout panic kept the macOS CI step open for about 3.5 h after the panic** (#1132, run 40234187614, the first run of the job). The bridge test binary panicked at 21:34:06, and the step ended only at the cancel at 01:07:54. `go test` had stopped: its `FAIL` line for `internal/bridge` is in the log, and `test-rusage.ndjson` has `exit_code` 2. The step shell waited on a capture of `make test-integration`:
  `status=$( { { go test ...; echo $? >&3; } | tee ... | testlatency -render >&2; } 3>&1 )`.
   Descriptor 3 of that capture went from `go test` to `testrusage`, to the test binary and to the tmux client. The daemon `tmux` server then kept it.
  - The proof ran on 2026-10-26 on a private socket under `/tmp`, never on the shared server. A test package starts a tmux session and then sleeps past `-timeout 3s`. It runs in the same capture shape. Without the fix, the capture stayed open for 25 s until the private server was stopped by hand. `lsof` showed the server with `3 PIPE`. With the fix, the capture ended in 6 s, and the guard had stopped the server.
  - Go 1.23 `go test` sets `WaitDelay` on the test binary: a tenth of `-timeout`, and 5 s or more (`src/cmd/go/internal/test/test.go:816` of Go 1.23.12, the CI toolchain). Thus its own pipe was not the cause. The capture pipe of the shell has no such limit.
  - The fix has two layers, because each layer closes a different gap. The descriptor hygiene keeps each child of the test binary off the capture pipe, also a headless child that tmux does not own. The orphan guard stops the server and its panes, so they do not stay on the host.
  - Rejected option: close descriptor 3 for `go test` in `make test-integration`. That change protects one caller only, and each other caller of `go test` keeps the fault.
  - Rejected option: the orphan guard alone. A headless child of the test binary then still holds the pipe.
