# internal/tmuxtest

## Purpose

`internal/tmuxtest` gives a test binary that runs real tmux a tmux server that only its own process owns. A package adopts it with one line:

```go
func TestMain(m *testing.M) { os.Exit(tmuxtest.Main(m)) }
```

`internal/bridge` (in an external `bridge_test` file, because this package imports `bridge`) and `internal/looppreflight` adopt it. `cmd/evolve` isolates itself differently, by pointing `TMUX_TMPDIR` at a temporary directory in its own `TestMain`, and does not need it.

## Design

`Main` does four things, in order:

1. It sets `EVOLVE_TMUX_SOCKET` (`bridge.TmuxSocketEnv`) to `bridge.DeriveTestSocket(os.Getpid())`, `evolve-bridge-t<pid>`, replacing any inherited value. Every bridge tmux call goes through `bridge.TmuxSocketArgs`, so every session the tests create, and every child process they spawn, lands on that socket. A test run inside a live loop's pane no longer inherits the loop's `evolve-bridge-p<pid>` socket.
2. If tmux is on `PATH`, it stops any server already on that socket (a server left by a dead process whose pid was recycled), then starts the server itself in one tmux invocation: `-f /dev/null start-server`, `set-option -s exit-empty off`, `set-option -g default-shell /bin/sh`, `set-option -g default-command "exec /bin/sh"`.
3. It runs the tests.
4. It stops that server with `swarm.ExecKillServer`, which also removes the socket file. It never touches another socket.

Why each server option:

- **The process starts its own server.** A tmux server's global environment is the environment of the client that started it. On the shared default socket, one server lived from 2026-09-30 to 2026-10-06, kept alive by sessions a killed test had leaked. Every later test pane on the host ran with that dead process's `GOCOVERDIR`, working directory, terminal session id and `PATH`.
- **`/bin/sh` is the pane shell.** Without it, every pane started the operator's login shell. On the development host `~/.zprofile` runs `pyenv init` twice, and each run takes pyenv's host-wide rehash lock. Concurrent login shells queue on that lock: 1, 8 and 16 shells at once took 0.33, 1.91 and 3.72 s. A pane became ready in about 0.5 s on a quiet host and up to 3.2 s under concurrency. A `/bin/sh` pane took about 0.11 s, and at most 0.36 s under the same concurrency. The real-tmux fixtures drive a scaled boot budget of about 8 s, so two floors at once pushed the boot over it.
- **`-f /dev/null`** keeps the operator's `.tmux.conf` out of the tests.
- **`exit-empty off`** keeps the server alive between tests. Otherwise the next test's client would start a new server, with the user config and the login shell.

`StartServer(socket)` and `StopServer(socket)` are the steps 2 and 4 alone (exported 2026-10-09). A test that must kill a server uses them on a private socket of its own, never on the shared socket of the binary. For example, `TestRealTmux_AKilledServerEndsTheWatchingDispatchAsPaneLostWithinOneLivenessInterval` uses `evolve-bridge-t<pid>-panelost`. It starts that server, kills it during a dispatch and stops it in cleanup ([cycle 1853](../../incidents/cycle-1853-secondary-defects.md)). `StartServer("")` refuses before it starts anything, because an empty name selects the default server. Pinned by `TestStartServer_RefusesAnEmptySocketBeforeItStartsAnything`.

A test binary whose tmux server cannot start fails with exit 1 and runs nothing; it never falls back to the shared server. A test binary on a host without tmux still gets its own socket name, and the real-tmux tests skip as before.

## Invariants

- **One server per test process, named by its pid, and killed only by its owner or by the dead-owner GC.** `swarm.ReapOrphanSockets` reaps `evolve-bridge-t<pid>` only when that pid is dead, the same liveness gate it applies to a loop's `evolve-bridge-p<pid>`. A test binary killed by its timeout skips step 4, and its server is reclaimed by the next `evolve gc` or loop preflight. Pinned by `TestMain_RunsTheTestsOnAFreshServerOwnedByThisProcess` (the socket, the pane shell, `exit-empty`, the stale server replaced, the runner's exit code returned, the socket gone afterwards) and, in `internal/swarm`, `TestReapOrphanSockets_ReapsADeadTestProcessSocketAndSparesALiveOne`.
- **It fails loudly, never falls back.** Pinned by `TestMain_AServerThatCannotStartRunsNothingAndFails` and `TestMain_WithoutTmuxStillRunsTheTestsOnTheirOwnSocketName`.
- **Concurrent test processes never share a server.** Pinned end to end by `internal/bridge`'s `TestRealTmux_ConcurrentTestProcessesNeverShareATmuxServer`, which runs two copies of the bridge test binary at once.
- **Test code only.** `internal/repocontract`'s `TestTestOnlyHelpersAreNeverImportedByProductionCode` fails on any non-test file that imports this package, since `Main` starts and kills tmux servers and rewrites `EVOLVE_TMUX_SOCKET`.
- **No defer before exit.** `Main` returns its code, and the adopting `TestMain` calls `os.Exit` on it, so the `testmainexit` analyzer has nothing to flag.

## Findings

- **Real-tmux reds whenever another process ran bridge tests at the same time** (inbox `realtmux-tests-interfere-across-concurrent-test-processes` and the earlier `realtmux-bridge-tests-flake-under-package-parallel-load`). Two apicover-style floors run at once, with a logging `tmux` shim on `PATH`, both went red in `internal/bridge` alone. Per-session boot latency rose from about 0.5–1 s to 3.4–8.3 s, and `TestRealTmux_E2E_LiveInjection_UnblocksAgent`'s 8 s injection sender gave up before the prompt was delivered. No test or reaper killed the sessions. Two causes stacked: the shared, stale, login-shell server described above (this package's part), and, dominant, macOS's first-exec scan of the fake CLI scripts the fixtures wrote and then executed directly (10–15 s per first exec while another floor ran). The bridge fixtures now install their fakes with `fakeclitest.Install` ([internal-fakeclitest.md](internal-fakeclitest.md)), a hard link to the already-scanned test binary that runs the script through its interpreter. Isolation alone was not enough: a verification floor run beside two more floors, before that fixture change, still timed out every directly executed fake. The full record is the [incident](../../incidents/2026-10-06-realtmux-tests-share-a-stale-tmux-server.md).
