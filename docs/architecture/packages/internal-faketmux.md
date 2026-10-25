# internal/faketmux

## Purpose

`internal/faketmux` stands in for the `tmux` binary in hermetic end-to-end tests. With it, a tmux driver of the bridge (`agy-tmux`, `agy-claude-tmux`, `codex-tmux`) runs a full simulated cycle with no tmux server and no real LLM.

The substitution matrix of [llm-isolation.md §14.1](../llm-isolation.md) uses it for rows M2 to M4. Before X1b, the matrix put a stub `tmux` that always failed on the row `PATH`. Thus each tmux row stopped at `new-session: exit status 1` on each host. A real tmux on `PATH` was no answer: the run then depends on the host, and it starts a tmux server on the host.

Spec: [llm-isolation.md §14.1](../llm-isolation.md). Inbox item: `isolation-matrix-tmux-repl-fake`.

## Design

**Entry.** `evolve-fake-cli --fake-tmux <state-root> <tmux arguments>` calls `faketmux.Run`. The matrix installs a `fakeclitest` stub named `tmux` whose script runs that command line. The stub writes the state root into its script. The bridge calls tmux with an allowlisted environment (`bridge.tmuxClientEnv`), so an environment variable does not arrive. `faketmux.Flag` is the flag word.

**The command line.** `Run` reads the global flags `-L <socket>` and `-f <file>`, then splits the rest on `;` into tmux commands. Each command has its own set of flags that take a value. These commands are served, because the bridge, `swarm` and `cmd/evolve` send them (`go/internal/bridge/tmux.go`):

| Command | What the fake does |
|---|---|
| `start-server`, `kill-server` | make the server directory; kill each pane and remove the directory |
| `new-session -d -s -x -y -c [cmd...]` | start the pane process (one argument runs through `/bin/sh -c`, no argument runs `/bin/sh`) |
| `has-session`, `kill-session`, `resize-window` | find the live session; kill its process group; succeed |
| `set-option -t`, `show-options -qv -t` | keep a session option as a file; print it (an unset option prints nothing) |
| `send-keys [-l]`, `load-buffer -b`, `paste-buffer -b [-d]` | write the keys or the buffer to the pane input, and echo them to the screen |
| `capture-pane -p [-S -N or -S -]` | print the last `-y` rows of the screen, plus `N` rows of scrollback, or all of it |
| `display-message -p`, `list-panes -F`, `list-sessions -F` | expand `#{session_name}`, `#{pane_pid}`, `#{pane_current_command}` and `#{window_activity}`; another variable expands to nothing |

An unknown command or flag fails with exit 1 and a `faketmux:` line on stderr, as tmux does.

**The pane.** A session is a directory `<state-root>/<socket>/sessions/<name>/` with these files:

- `in`, a FIFO. The pane process gets it as its standard input, opened for read and write. Thus the pane never reads end-of-file when a writer closes, as a terminal does not.
- `screen`, a file. The pane writes its standard output and its standard error to it.
- `pane`, the record `<pid> <rows>`.
- one `option-<hex name>` file for each option.

The pane process starts in its own session (`Setsid`), so `new-session` returns at once and `kill-session` kills the process group. The pane gets `TERM=screen` and does not get the outer `TMUX`, `TMUX_PANE` or `TERM`.

**The terminal.** The fake copies the parts of a terminal in cooked mode that the drivers read:

- `Enter` (and `C-m`) sends a newline, as the `ICRNL` setting of a terminal does. Thus `/bin/sh` runs a line, and the fake REPL reads it.
- Text keys and pasted buffers are echoed to the screen before the pane reads them. The submit check of the bridge reads the echoed prompt (`driver_tmux_submitverify.go`).
- `paste-buffer` writes the buffer as it is. No program in the fake asks for bracketed paste, so `-p` adds no markers.
- `capture-pane` turns `\r\n` into `\n` and drops a lone `\r`. It does not wrap long lines.

**The foreground program.** `#{pane_current_command}` reads `ps -A -o pid= -o ppid= -o comm=` and follows the newest child from the pane process to the deepest descendant. The dead-shell guard of the bridge (`paneShellProcess`) thus sees `sh` before the launch and the CLI after it.

**A session ends with its process.** `has-session` succeeds only while the pane process lives (`kill(pid, 0)`). A session whose program exited is gone, and its name is free for a new session.

**Writes block like a terminal.** A writer opens the FIFO without blocking, so a pane with no reader fails at once (`ENXIO`), not forever. Then it sets the descriptor to blocking, so a prompt larger than the pipe buffer waits for the reader. The first matrix run without this step failed with `resource temporarily unavailable` on each prompt paste.

## Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Fake the `tmux` binary, not the `TmuxController` interface | The matrix runs `evolve cycle run` as a child process, so an in-process `bridge.FakeTmuxController` cannot reach it. The binary is the seam that the child already resolves through `PATH`. | A test seam in `bridge` that selects a fake controller from the environment: it adds a control flag to production code (ADR-0156). It also touches the bridge, which X3 and X4 change in parallel. A private real tmux server per row: the result then depends on the host, and CI runners do not all have tmux. |
| D2 | A FIFO plus a screen file, no pseudo-terminal | The standard library has no pseudo-terminal, and the drivers read only the screen text, the echo and the pane process. | A pseudo-terminal module: a new dependency for no check that the matrix makes. |
| D3 | Serve the fake from `evolve-fake-cli` | The matrix already builds that binary, and its job is to stand in for a CLI. | A new `cmd/evolve-fake-tmux`: a second build in each run, and a `main` function that coverage cannot reach. |

## Invariants

- **A pane runs what it is sent and shows the output.** Pinned by `TestRun_APaneRunsTheKeysItIsSentAndShowsTheirOutput` and `TestRun_APastedBufferReachesTheProgramAndTheBufferIsDeletedWithD`.
- **`kill-session` and `kill-server` end the pane processes.** Pinned by `TestRun_KillSessionEndsThePaneAndTheSessionIsGone` and `TestRun_KillServerEndsEverySessionAndListSessionsThenFinds_NoServer`. The fake signals only a recorded pid above 1. The signal goes through the `signalGroup` seam, so a test never sends a real kill to a wrong group.
- **A refusal is loud.** Pinned by `TestRun_RefusesWhatTmuxWouldRefuse` and `TestRun_TheStateRootAndTheSocketNameMustBeUsable`.
- **No platform branch.** The package has no branch on `runtime.GOOS`. It uses POSIX calls (`mkfifo`, `setsid`, `kill`) and `ps -A -o`, which macOS and Linux both have. The `faults_test.go` seams (`openFile`, `signalGroup`, `setBlocking`, `runPS`) reach each error path on each OS.

## Limits

- The fake is not a terminal emulator. It ignores the pane width, cursor moves, colors and the alternate screen. A driver check that reads these needs a real tmux; the bridge real-tmux tests own it (`internal/bridge/tmux_repl_integration_test.go`).
- `C-c` sends the byte `0x03` and no signal, because the pane has no terminal driver.
- A blocked write to a pane that does not read waits. The fake sets no deadline of its own, because the bridge runs each tmux call with a 30 s bound (`tmuxCmdTimeout`, `go/internal/bridge/tmux.go`).
- `has-session` reads `kill(pid, 0)`, which succeeds for a zombie. An orphaned pane is reaped by the init process of the host. In a container whose first process does not reap, a dead pane stays a session.
- The `pane` record holds a pid and no start time. A pid that the host gives again to an unrelated process makes the old session look alive. The record lives only for one test run, so this window is small.
