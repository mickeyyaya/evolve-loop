# The agent sandbox contract

> Status: live since 2026-10-10. Owner: `internal/bridge` and `internal/adapters/sandbox`. The test is `TestAgentSandboxContract` in `go/internal/bridge/sandbox_contract_integration_test.go`.

## Purpose

This document gives the capability contract of a phase agent on macOS. Each row tells what the agent can do and what the agent cannot do in its sandbox. One table-driven test checks each row under the real `sandbox-exec`. The test does not simulate the sandbox.

The operator asked on 2026-10-09: "should we simulate sandbox or design test to run in sandbox environment to cover this edge cases?" The answer is the real sandbox. A simulation cannot find a kernel rule that SBPL text does not show.

## How the test makes the profile

The test uses the same path as a live builder launch:

1. It reads the sandbox block of `.evolve/profiles/builder.json` with `profiles.Loader`.
2. It resolves `deny_subpaths` with `resolveSandboxDenials`, as `launch.go` does.
3. It builds a `Config` and calls `sandboxPrefixForLaunch`.
4. `defaultSandboxWrapWithProbe` makes the profile with the measured probe. The output is `sandbox-exec -f <file>`.

Thus each row checks the profile that `GenerateSBPL` makes from the real wrapper inputs. The terminal path is the `pane_tty` of a real tmux pane.

The test uses a private `TMUX_TMPDIR` in `/tmp`. It starts two tmux servers there: the run server and the `default` server. It never connects to a socket of the operator or of a live loop.

## The contract

Each row runs one shell command in the profile. A row has one of three verdicts:

- **allow:** the command must exit 0.
- **deny:** the command must fail, and its output must contain `Operation not permitted`. The text is the evidence that the sandbox refused the command. A failure for another reason does not pass the row.
- **gap:** the command succeeds today, but the contract says deny. The row fails when the gap closes, so that the author changes the row to deny.

After each row, the host must still reach the run server with its sessions `agent` and `victim`.

| Action | Verdict | Evidence |
|---|---|---|
| Start a private tmux server (`-L`) and run a window | allow | The window prints its own `/dev/ttysN`. It is not the pane tty or the peer tty. |
| Connect to the run socket | deny | `tmux -L <run>` gets `Operation not permitted`. |
| Connect to the `default` socket | deny | A bare `tmux` gets `Operation not permitted`. |
| `tmux kill-server` against the run server | deny | The run server keeps its sessions. |
| Rename the run socket | deny | `mv` gets `Operation not permitted`. |
| Hard-link the run socket | deny | `ln` gets `Operation not permitted`. |
| Unlink the run socket | deny | `rm` gets `Operation not permitted`. |
| Connect through a symlink to the run socket | deny | The symlink is made. The connection gets `Operation not permitted`. |
| Rename the socket directory | deny | `mv` gets `Operation not permitted`. |
| Make and remove a file in the socket directory | allow | `touch` and `rm` succeed. |
| Signal a host process (the run server) | deny | `kill` gets `Operation not permitted`. |
| Signal its own child | allow | The child `sleep` stops with a signal status. |
| Write in its own worktree | allow | The file reads back. |
| Write in its own cycle run directory | allow | The file reads back. |
| Write in another cycle's `.evolve/runs/cycle-M` | gap | The write succeeds today. See "Known gap". |
| Write in the protected `.evolve/events/` | deny | The write gets `Operation not permitted`. |
| Write to another pane's tty | deny | The open for write gets `Operation not permitted`. |
| Read another pane's tty | deny | The open for read gets `Operation not permitted`. |
| Open its own pane tty for read and write | allow | Both opens succeed. |
| Incident cycle 1853: the exact probe with `TMUX` unset | allow | The probe prints `rc=0` and `own-server`. The run server survives. |
| Incident cycle 1853: the exact probe with `TMUX` set to the run server | allow | The probe reaches no server. The run server survives. |

## The threat model

The agent is a model that runs commands in a pane. The agent is not hostile by intent, but it does wrong things. In cycle 1853 the builder killed the run tmux server two times with a probe ([incident](../incidents/cycle-1853-tmux-run-server-kill.md)). The contract protects these assets:

- **The run tmux server and its panes.** A loss stops every phase of the wave. The socket rows, the signal row and the incident rows protect it.
- **The other panes.** A pane tty carries the keystrokes and the output of another agent. The tty rows protect it. Before 2026-10-10 the profile granted a read of all of `/dev`, and the read of another pane's tty was not denied.
- **The shared state of the run.** `.evolve/events/` and the run directories of other cycles hold the evidence that the loop trusts. The write rows protect it.
- **The useful work.** An agent must run `go test`, `git` and `tmux` with a private server. The allow rows protect it. A profile that is too strict causes a red test, and a red test caused the cycle-1853 probe.

The contract does not cover these items:

- the network: a source-writing phase gets the network, because the model API needs it;
- Linux: `bwrap` has a separate model (a new `/tmp` and a new PID namespace);
- a launch without the sandbox (see "Residual risks" in the incident).

## How the `/dev` grant works

The profile grants a read of `/dev` without the terminals:

```
(allow file-read* (require-all (subpath "/dev") (require-not (regex #"^/dev/(tty|pty)[^/]+$"))))
```

These grants then give back the terminals that the agent needs:

- the pane's own terminal and `/dev/tty`: `(allow file-read* file-write-data (literal "/dev/tty") (literal <pane>))`;
- the agent's own pseudo-terminals: a read, write and ioctl grant on `/dev/ttys[0-9]+`, scoped by the extension `com.apple.sandbox.pty`;
- `/dev/ptmx`, to open a new pseudo-terminal.

`/dev/null`, `/dev/zero`, `/dev/random`, `/dev/urandom`, `/dev/dtracehelper` and `/dev/fd` do not match the regex, so they stay readable. The regex does not match `/dev/tty` itself, so a headless agent can still open `/dev/tty`.

Two results of this rule are known and accepted:

- **The serial devices stay readable.** `/dev/cu.*` does not match the regex. The risk is low: a pane does not use a serial device, and an agent rarely has a reason to open one.
- **A headless agent cannot read a tty device.** A headless launch has no pane terminal, so no literal grant gives back a `/dev/ttysN` device. The agent can still open `/dev/tty` and its own pseudo-terminals.

## Known gap

Each profile declares `.evolve/runs/cycle-*` in `sandbox.write_subpaths`. `resolveSandboxWriteGrants` grants the longest ancestor without a glob, which is `.evolve/runs`. Thus an agent can write in the run directory of each other cycle. The workspace of the phase already grants its own cycle directory. The fix changes about 90 profiles and is not in this lane. The row stays `gap` until that fix lands.

## How to add a row

1. Find the command that the agent ran, or the action that you must allow or deny.
2. Add one `contractRow` to `agentContract` in `sandbox_contract_integration_test.go`. Give the action, the verdict and the script.
3. If the row needs more evidence, add a `check` function.
4. If the row needs an environment variable, put it in `env`. `<run>` becomes the `TMUX` value of the run server.
5. Run the test before you change the profile. A new deny row must fail first.
6. Add the row to the table in this document.

An incident command stays in the table as a row forever. Do not remove a row to make the test pass.

## Related tests

- `TestCI_TheRunnerCanApplyTheAgentSandbox` runs first in `internal/bridge`. It applies the generated agent profile to `/usr/bin/true`. If the runner cannot apply a sandbox, the message gives the binary path, the apply error, its output and the SBPL.
- `TestPaneParity_RealTmuxPackagesPassInTheSandboxedPaneAsOnTheHost` runs `./internal/swarm` and the real-tmux tests of `./internal/bridge` two times: on the host, and in a real pane through `bootTmuxREPL` with the generated profile. A test that passes on the host and fails in the pane fails the gate. It also checks the pane environment: no `TMUX`, no `TMUX_PANE`, and the exports of the boot. Each run must be evidence. The exit status agrees with the test results, and each line is test2json. The host passes each test that the target names.
- `TestPaneParity_ReportsTheCycle1853TriggerWhenThePtyRulesAreRemoved` removes the pty rules from the profile. The harness must then report `TestExecTmuxKill_NeverKillsASessionWhoseNameExtendsTheTarget` as host=pass and pane=fail.
- In CI on macOS, a real-tmux test or a sandbox test fails when `tmux` or `sandbox-exec` is missing ([internal-testhost](packages/internal-testhost.md)).
