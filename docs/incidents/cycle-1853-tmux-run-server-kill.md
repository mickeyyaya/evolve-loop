# Incident: a builder agent killed the run's tmux server (cycle 1853)

**Date:** 2026-10-09 | **Wave:** 89 | **Severity:** INCIDENT (the waves stopped) | **Status:** fixed on branch `fix/pane-tmux-isolation`

## What happened

The builder agent of cycle 1853 ran this command in its bridge pane to probe tmux:

```sh
d=$(mktemp -d /tmp/swxprobe.XXXX)
TMUX_TMPDIR=$d tmux -f /dev/null new-session -d -s probe
TMUX_TMPDIR=$d tmux kill-server
```

The agent expected a private tmux server under `$d`. But the pane shell had `TMUX=/private/tmp/tmux-501/evolve-bridge-p<loop-pid>,...`. A tmux client without `-L` or `-S` uses the socket in `$TMUX` and ignores `TMUX_TMPDIR`. Thus `new-session` made the session `probe` on the run server, and `kill-server` killed the run server. The pane of the agent and each other pane on that server stopped.

## Timeline (UTC)

| Time | Event |
|---|---|
| 09:21:08 | The builder agent of cycle 1853 runs the probe. The run server stops. |
| 10:02:17 | The same agent runs the probe again. The run server stops again. |
| After 10:02 | The operator stops the waves. The investigation reproduces the kill on a private socket (`probe.sh`: `TMUX` is set in the pane, `rc=0`, `kill rc=0`). |
| 2026-10-09 | The fix in this branch. The pane is detached from the run server. The sandbox denies the sockets and the signals. The agent rules forbid the command. |

## Why it was possible

1. `driver_tmux_boot.go` did not remove `TMUX` or `TMUX_PANE` from the pane shell. tmux sets both in each pane.
2. The identity block of the pane told the agent to run `tmux display-message -p '#S'` and to look for its session in `tmux ls`. Both commands use `$TMUX`, so the agent learned that tmux in its pane reached the run server.
3. The sandbox profile grants read and write on `/private/tmp` and allows the network for a source-writing phase. Nothing denied a connection to the run socket.

## The fix

1. **The detach.** The boot sends `unset TMUX TMUX_PANE` before the CLI launch. A tmux client of the agent goes to its own server. If that send fails, the boot stops and the CLI does not start.
2. **No connection.** For each phase agent, `GenerateSBPL` denies `network-outbound` to the resolved run socket and to the user's `default` socket. After the unset, a bare `tmux` client goes to `default`. The bridge runs outside the profile, so it drives the server as before.
3. **No move.** The profile now denies `file-write*` on the two sockets and on the socket directory.
   - The profile grants writes in `/private/tmp`, so this denial is necessary.
   - The rule on the directory is a `literal` rule. The agent can still make files in the directory.
   - A rename, a hard link or an unlink of the socket fails. A rename of the directory fails.
   - A symlink is allowed. SBPL checks the resolved path, so the connection through the symlink is refused.
4. **No signal.** The profile allows signals only to processes in the same sandbox (`(allow signal (target same-sandbox))`). `kill` or `pkill tmux` cannot stop the run server, the bridge or another pane. The agent can still stop its own children.
5. **The rules.** The identity block no longer names `tmux display-message` or `tmux ls`. The authority block forbids three things:
   - `tmux` without your own `-L <private-socket>`;
   - `tmux kill-server`;
   - a kill, pkill or killall of `tmux` or of a process you did not start.
6. **Refusals.** If `TMUX_TMPDIR` is relative, or the socket directory cannot be resolved, the bridge does not wrap the agent.

Design notes: [internal-bridge.md](../architecture/packages/internal-bridge.md) ("A phase agent cannot reach the run's tmux server").

## The proofs

- `TestRealTmux_AgentTmuxProbeCannotKillTheRunServer` runs the exact probe in a real pane, through the real boot, beside a victim session, on the test socket `evolve-bridge-t<pid>`. Before the fix, the victim session was gone and the boot timed out (exit 80). After the fix, the victim stays and the probe still gets its own private server.
- `TestGenerateSBPL_RunSocketIsDeniedToTheAgentAndOpenToTheHost` runs each attack under the real `sandbox-exec` profile: connect, rename, hard link, symlink, unlink and directory rename. Each one fails, and after each one the host still reaches the server. The agent can still write in the socket directory and drive a private server of its own.
- `TestGenerateSBPL_TheAgentSignalsOnlyItsOwnProcesses` shows that the agent cannot signal the tmux server and can still stop its own `sleep` child.

## Why the retry ran the probe again

The second kill (10:02:17) was a replay of the first. Each step of this chain was verified from the transcripts of the two build attempts:

1. The task was the swarm pgid fix, so the builder ran `go test ./internal/swarm`.
2. `TestExecTmuxKill_NeverKillsASessionWhoseNameExtendsTheTarget` failed in the pane: `kill_exact_test.go:37: new-session evolve-bridge-r1-c5-build-pid421: exit status 1`.
3. The cause was the sandbox profile, not the code.
   - `GenerateSBPL` did not allow a pseudo-terminal, and tmux makes a window with `forkpty()`.
   - The server failed with `create window failed: fork failed: Operation not permitted`.
   - Thus each real-tmux test was red in each sandboxed pane. Nothing told the agent so.
4. The agent saw that its diff did not touch the test or `kill.go`. It concluded that the failure was environmental. It then probed the isolation of the test by hand (see "What happened").
5. The pane had `$TMUX`, so the probe reached the run server and killed it.
6. The runner then dispatched the build again with a byte-identical prompt. The worktree kept the diff of the first try. Thus the same test failed in the same way, and the agent ran the same probe. The second prompt had no facts about the first try: how it ended, when, and its last commands. Lane R2 (`attempt postmortem`) adds these facts to the next prompt.

### The trigger fix (lane R1)

The agent profile now lets the agent open its own pseudo-terminals and no other terminal. `writeTerminalRules` (`go/internal/adapters/sandbox/sandbox.go`) writes three rules into each agent profile:

- `(allow pseudo-tty)`;
- `(allow file-read* file-write* file-ioctl (literal "/dev/ptmx"))`;
- `(allow file-write-data file-ioctl (require-all (regex #"^/dev/ttys[0-9]+$") (extension "com.apple.sandbox.pty")))`.

The kernel gives the sandbox extension `com.apple.sandbox.pty` only for the slave of a pseudo-terminal that the sandboxed process opened. Apple's `application.sb` uses the same rule. The grant does not include another pane's terminal, because the agent did not open it. A write to the host pane's tty from the agent's own window fails with `Operation not permitted`.

A grant of `/dev/ptmx` alone is not enough. The server then opens the master but cannot open the slave `/dev/ttysN`. The first bisect showed a pass only because the new slave was `/dev/ttys003`, which was the pane terminal of the cycle-1853 profile. A live test with a profile that has only the `/dev/ptmx` rule fails with `fork failed`.

The rules are in each agent profile, not only in source-writing phases. Read-only phases also run tests, and a red real-tmux test in a read-only phase can cause the same probe.

After #836 a private server of the agent is safe. The run socket and the `default` socket stay denied. Signals reach only the same sandbox. With the fix, the real-tmux tests are green in a sandboxed pane, so the trigger of the probe is gone.

Proofs:

- `TestGenerateSBPL_GrantsOnlyThePseudoTerminalsTheAgentOpens` (unit): the three rules occur once, and the only pattern rule is the slave rule that the extension scopes.
- `TestGenerateSBPL_TheAgentWindowGetsItsOwnTerminalAndNoOtherPane` (integration, real `sandbox-exec`): the agent starts a private server, and its window runs on its own pty. A write to the host pane's tty fails.
- `TestGenerateSBPL_TheCycle1853ProbeGetsItsOwnServerAndLeavesTheRunServerAlive` (integration): the exact probe, with `TMUX` unset as the boot does, gets its own server, and the run server keeps its session.
- Live: `go test ./internal/swarm` in `sandbox-exec` with the generated profile fails before the fix at `kill_exact_test.go:37` and passes after it.

## Residual risks

- **Phases without the OS sandbox.** `sandboxPrefixForLaunch` wraps a tmux launch only when it has a worktree or `RequireSandbox` is set. These tmux launches have neither, so they get the detach and the rule, but no profile:
  - A phase dispatch in single-driver mode (no `EVOLVE_FLEET`) with no worktree. `prepareTmuxREPL` then uses the process working directory. This occurs in three cases:
    - the cycle worktree was not made outside fleet mode (`cyclerun.go` records the failure and keeps `ActiveWorktree` empty);
    - the router and advisor dispatch while `ActiveWorktree` is empty;
    - `evolve subagent run` without `--worktree`.
  - In fleet mode, the driver refuses a launch with no worktree (`errWorktreeRequired`).
  - The recipe sessions (`newRecipeDriver`: the `/model` picker capture, the `clicontrol` usage and status captures, the recipe engine). They never call the sandbox wrapper. They also do not send the unset line. The bridge types only slash commands into them, not a task prompt.
  - Boot smoke and live smoke get a scratch worktree (`applyScratchCwd`), so they are wrapped.
- **No sandbox at all.** Then the agent has only the detach and the rule. This occurs with `EVOLVE_SANDBOX=off`, in a nested session where the sandbox does not apply, and on a host with no sandbox binary.
- **Linux.** The profile rules are macOS SBPL. `bwrap` mounts a new `/tmp` and puts the agent in a new PID namespace. Thus the default socket directory is not visible and the host processes cannot be signalled. A run with `TMUX_TMPDIR` outside `/tmp` has no Linux socket denial.
