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
- **isolation row:** a row that names a row id of the conformance matrix of [llm-isolation.md](llm-isolation.md) §14. The test gives the result of the allow or deny check to `isolation.Judge`. An open row must fail its check with the cause that the row names. The test fails when the check passes or fails for another reason. The component that closes the gap then marks the row green in `go/internal/isolation/matrix.go`.

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
| Write in another cycle's `.evolve/runs/cycle-M` | deny | The write gets `Operation not permitted`, and no file is made on the host. See "The run directory grant". |
| Write in the protected `.evolve/events/` | deny | The write gets `Operation not permitted`. |
| Write the host resume witness in `.evolve/resume-witness/` | deny | The write gets `Operation not permitted`. A resume trusts this file ([ADR-0039 D4](adr/0039-failure-floor-and-failure-signal-contract.md)). |
| Write to another pane's tty | deny | The open for write gets `Operation not permitted`. |
| Read another pane's tty | deny | The open for read gets `Operation not permitted`. |
| Open its own pane tty for read and write | allow | Both opens succeed. |
| Incident cycle 1853: the exact probe with `TMUX` unset | allow | The probe prints `rc=0` and `own-server`. The run server survives. |
| Incident cycle 1853: the exact probe with `TMUX` set to the run server | allow | The probe reaches no server. The run server survives. |
| Isolation N3: exec an LLM binary (a `claude` stub) with `--version` | deny (green, X4) | The shell gets `Operation not permitted`. The profile is built at `isolation.i3.stage=enforce`. |
| Exec an older installed version of the `claude` stub under the install root (`$HOME/.local/share/claude/versions/2.1.304`) | deny | The exec gets `Operation not permitted`. The `install_roots` entry of the claude manifests covers each version. |
| Re-enter the profile through `sandbox-exec` to exec the `claude` stub | deny | The exec of `sandbox-exec` gets `Operation not permitted`. |
| Isolation N5: read the raw output `<ws>/.bridge/scout-stdout.log` | deny (green, X5) | The read gets `Operation not permitted`. The profile is built at `isolation.i5.stage=enforce`. |
| Read a raw log that the bridge left in the own workspace (`build-stdout.log`) | deny | The read gets `Operation not permitted`. The raw logs stay in the workspace, and a rule denies them by name. |
| A retry opens the retrospective of its predecessor through `inputs/index.json` | allow | The read returns the lesson. `inputindex.Build` writes the index from fixture `lane-scope.json` records, so cycle 6 is in the lineage of cycle 7. |
| Isolation N7: a retry reads the `audit-report.md` of an unrelated cycle | deny (green, X5) | The read gets `Operation not permitted`. Cycle 9 built another item, so the index does not list it. |
| Isolation N8: read `~/.ssh/id_fixture` in the test `HOME` | deny (green, X5) | The read gets `Operation not permitted`. The file is a fixture, not a host secret. |
| The Claude OAuth login reads its Keychain item through the security service | allow | `security find-generic-password` returns the item `Claude Code-credentials` of a fixture keychain in the test `HOME`. |
| Write the pipeline-written `inputs/index.json` | deny | The write gets `Operation not permitted`. |
| Rename a raw log of the own workspace to a name that no rule matches | deny | The rename gets `Operation not permitted`. |
| Hard-link a raw log of the own workspace under another name | deny | The link gets `Operation not permitted`. |
| Rewrite the `lane-scope.json` of the own workspace | deny | The write gets `Operation not permitted`. |
| Write a pipeline lineage record in `.evolve/lineage/` | deny | The repo is read-only for the cell. |
| Isolation N17: a cell appends a record to the access log `.evolve/context-access/cycle-7.jsonl` | deny (green, CX3) | The write gets `Operation not permitted`. No write grant can cover the log ([cell-context-index.md](cell-context-index.md) §8). |
| Isolation N16 (its own test): a cell reads the indexed retrospective of cycle 6 | allow (green, CX3) | `log show` returns the report of the grant rule `rule=grant.index`. The pipeline writes one access record of kind `retrospective` with `source: sandbox`. |
| Isolation N9: open a TCP connection to a loopback listener of the host | allow (green) | `nc -z` connects. The row records that the network stays open. |
| Isolation N10: an agy pane writes the protected paths `go/internal/guards/role.go`, `.claude/settings.json` and `go/internal/phases/audit/probe_quarantine.go` in its worktree (its own test, `TestAgentSandboxContract_N10_AnAgyPaneCannotWriteAProtectedPath`) | deny (X7) | Each write gets `Operation not permitted`. A write of the sibling `go/internal/guards2/x.go` goes through. |
| `kill(-1)` from a cell (a static row, `TestSandboxContract_KillMinusOneFromACellReachesNoHostProcess` in `go/internal/adapters/sandbox`, every OS) | deny (X7) | The row reads the generated profiles and runs no signal. The SBPL profile has `(deny default)` and one signal grant, `(allow signal (target same-sandbox))`. The `bwrap` prefix has `--unshare-pid`. Thus the signal reaches only the processes of the cell. |

## The threat model

The agent is a model that runs commands in a pane. The agent is not hostile by intent, but it does wrong things. In cycle 1853 the builder killed the run tmux server two times with a probe ([incident](../incidents/cycle-1853-tmux-run-server-kill.md)). The contract protects these assets:

- **The run tmux server and its panes.** A loss stops every phase of the wave. The socket rows, the signal row and the incident rows protect it.
- **The other panes.** A pane tty carries the keystrokes and the output of another agent. The tty rows protect it. Before 2026-10-10 the profile granted a read of all of `/dev`, and the read of another pane's tty was not denied.
- **The shared state of the run.** `.evolve/events/`, `.evolve/resume-witness/` and the run directories of other cycles hold the evidence that the loop trusts. The write rows protect it.
- **The useful work.** An agent must run `go test`, `git` and `tmux` with a private server. The allow rows protect it. A profile that is too strict causes a red test, and a red test caused the cycle-1853 probe.

The contract does not cover these items:

- a deny of the network: a source-writing phase gets the network, because the model API needs it. Row N9 records this allow;
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

## The run directory grant

Each profile declares `.evolve/runs/cycle-*` in `sandbox.write_subpaths`. Before 2026-10-20, `resolveSandboxWriteGrants` granted the longest ancestor without a glob, which is `.evolve/runs`. Thus an agent had write access to the run directory of each other cycle. The agent-messaging design (ADR-0129 D3) needs this isolation, because it finds the sender of an outbox file by its directory.

Now the resolver treats `.evolve/runs/cycle-*` as a template, like `{worktree_path}`. The template resolves to the workspace of the dispatch, which is the run directory of its own cycle. A subpath of the template stays inside that directory: `.evolve/runs/cycle-*/learn` becomes `<workspace>/learn`. Without a workspace, the grant is skipped. Each launch has a workspace, because `--workspace` is a required launch argument.

The resolver matches the template after it removes empty and `.` segments, so `./.evolve/runs/cycle-*` and `.evolve//runs/cycle-*/learn` are the same template. It keeps each `..` segment, so `.evolve/runs/cycle-*/../cycle-8` escapes its base and is refused.

Every other grant that reaches the run directories is refused with "covers the run directories of other cycles". Examples are `.evolve/runs/*`, `.evolve/runs/cycle-*x`, `.evolve/runs/cycle-8`, `.evolve/runs` and `.evolve`. The check ignores case, because the default macOS volume does too. Thus a new spelling cannot widen the grant again. A refused grant stops the launch, because the wrapper gives no sandbox and the profile requires one.

The `orchestrator` profile declared `.evolve/cycle-state*`, which widened to all of `.evolve`. It now declares `.evolve/cycle-state.json` and `.evolve/cycle-state.json.lock`.

| Rejected option | Why |
|---|---|
| A `{cycle}` template in each profile | It changes about 90 profiles, and a profile that keeps the old glob keeps the gap. The resolver fix closes the gap for each profile at one place. |
| The glob matched against the cycle number | The resolver does not get the cycle number. The workspace is already the own run directory. |

`TestEveryProfileWritesOnlyItsOwnCycleRunDir` checks each launchable real profile: no grant reaches another cycle, and each declared run-directory path is writable in the own run directory. A profile whose denials do not resolve cannot launch. The test requires that this set is exactly the `orchestrator` profile, which declares a glob denial.

The gap verdict stays in the test for the next known gap. No row uses it now.

## Known gap

The scout profile declares `.evolve/evals/cycle-*`. That glob is not a run directory, so it still widens to `.evolve/evals`. Thus the scout can write the evals of each other cycle. Inbox `sandbox-cross-cycle-evals-write` tracks the fix.


## The credential deny and the cell

The bridge denies writes in the parent of the dispatch store. Thus a cell cannot rename the store or put a symlink in its place ([llm-isolation.md](llm-isolation.md) §7.1.1). The deny is a `file-write*` subpath in SBPL and a `--ro-bind` mask in `bwrap`. A store in the wrong place can thus remove the write grant of the cell.

This happened on #1132 (run 40234187614, 2026-10-25). The bridge test store was `$TMPDIR/evolve-bridge-test-dispatch`, so the deny covered `$TMPDIR` and each integration workspace in it. The sandbox refused the artifact write of the fake agent, and the macOS CI step hung for about 4 h. [internal-bridge.md](packages/internal-bridge.md) "Findings" has the full chain.

Three rules now prevent a repeat:

| Rule | Where | Test |
|---|---|---|
| The wrapper keeps the deny, but reports a deny that is equal to the worktree or the workspace, or contains it. The report is a WARN line and an `isolation.violation` signal with `rule=launch.credential.covers-cell`. | `Deps.warnCredentialCoversCell` (`dispatch_credential.go`) | `TestSandboxWrap_ACredentialDenyOverTheCellKeepsTheDenyAndIsLoud`, `TestSandboxWrap_ANormalLayoutGetsNoCellWarning` |
| In the generated profile, the last rule that covers the workspace, the artifact or the worktree allows the write. | the profile that `defaultSandboxWrapWithProbe` makes | `TestSandboxProfile_NoCredentialDenyOrMaskCoversTheCellOfATestLayout`, `TestSandboxProfile_TheCellCheckSeesTheIncidentDeny` |
| The test store has the shape of `~/.evolve/dispatch`: `$TMPDIR/evolve-bridge-test-home/.evolve/dispatch`. | the `init` of `dispatch_credential_test.go` | `TestDispatchCredentialDeny_ThePackageStoreDeniesNoWriteOverTheTestWorkspaces` |

The profile test reads the rules in order, because the last rule that matches a path decides in both systems. A deny before a later allow is correct: the read-only repository deny comes before the grant of the worktree. The test checks two layouts. The first is the `itConfig` layout with the package default store. The second is the layout of this contract test, with `contractTempDir` and a store in the cell home.

The rejected option was to refuse the launch when the deny covers the cell. The deny protects the credential, and a refusal makes the deny optional for each layout that has the fault. The cell must stay without the write, and the report must make the cause visible at once.

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
- `TestIntegrationTests_EveryLaunchAndWaitHasABoundedContext` fails when an integration test gives `context.Background()` or `context.TODO()` to a launch or wait entry. A sandbox fault then fails one test in minutes, not at the package timeout ([internal-bridge.md](packages/internal-bridge.md), "Invariants").
- The tmux server of a test binary stops about 1 s after the binary dies, and no child keeps an inherited pipe ([internal-tmuxtest.md](packages/internal-tmuxtest.md)).
- In CI on macOS, a real-tmux test or a sandbox test fails when `tmux` or `sandbox-exec` is missing ([internal-testhost](packages/internal-testhost.md)).
