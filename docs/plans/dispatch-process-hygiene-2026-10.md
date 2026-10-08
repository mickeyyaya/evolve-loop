# Dispatch process hygiene: tag, reap and gc backstop (2026-10)

> **Status:** lane H2 (`cl-proc-hygiene`), 2026-10-08. The full design for logs and processes is [logging-and-process-hygiene-2026-10.md](logging-and-process-hygiene-2026-10.md). Lane H1 lands that document. This plan covers only the decisions K8, K9, K10 and K11 of that design.

## The request

The operator wrote this on 2026-10-08:

> "our gc should exame the possible zombie process generated through LLM CLIs are clean them up once the tasks or requests are completed / cancelled."

## The facts

| ID | Fact | Source |
|---|---|---|
| F1 | A dispatched agy pane is this tree: tmux, then `zsh`, then `agy`, then `node` (an MCP server). The `node` child inherits `EVOLVE_PROJECT_ROOT`. | `ps -E` on the host |
| F2 | No variable in the environment names one dispatch. | inventory of lane H1 |
| F3 | `tmux kill-session` closes the pane pty. The kernel sends SIGHUP to the foreground group only. Background groups and new sessions survive. | dossier F3.6 |
| F4 | macOS hides the environment of Apple binaries (`/bin/zsh`, `/bin/sleep`, `/usr/bin/tail`). Their arguments stay visible. A Homebrew `node` shows its environment. | local probe of `KERN_PROCARGS2` |
| F5 | `ps -E` prints the arguments and the environment on one line. An argument such as `grep EVOLVE_DISPATCH_ID=x` then looks like a tag. | `man ps` |
| F6 | 48 orphan `tail -F` processes from old console monitors ran for up to 24 days. gc did not see them. | host, 2026-10-08 |
| F7 | A test leaked a tmux session `evolve-bridge-it-cc2-3948` on the socket `evolve-bridge` for 7 days. gc skips it as `no-pid`. | `evolve gc --dry-run` |
| F8 | Since commit 576e56fc3 (2026-10-06), each test binary that uses tmux owns a server on the socket `evolve-bridge-t<pid>`. `TestMain` stops it. gc reaps a socket `evolve-bridge-t<pid>` whose pid is dead. | `internal/tmuxtest`, `swarm.ReapOrphanSockets` |
| F9 | 149 `exec.Command` and `exec.CommandContext` calls start processes in production code. Only 8 files set `WaitDelay`. | `grep` |
| F10 | The gc process rule proves ownership with ppid 1 and a cwd in a finished cycle tree. An MCP server usually has a cwd outside the tree. | `gc/processes.go` |

## Goals and non-goals

Goals:

- G1 (K8). Each dispatch has one id, `EVOLVE_DISPATCH_ID=<run>/<cycle>/<agent>/p<bridge-pid>n<nonce>`. Each process that the dispatch starts inherits it.
- G2 (K9). At the end of a dispatch (complete, fail, timeout or cancel), the bridge stops each process of the dispatch. A survivor is a loud `INCIDENT` signal.
- G3 (K10). Each `exec.Cmd` that the pipeline starts sets `Cancel` and `WaitDelay`. A test keeps it so.
- G4 (K11). gc stops what G2 did not stop. gc uses a proof of ownership for each process.

Non-goals:

- N1. Linux cgroups. A cgroup per dispatch is the only boundary that no descendant can escape (dossier F3.3). The host is macOS. This is a later item.
- N2. Log files and their retention. Lane H1 owns them.
- N3. `evolve logs`.
- N4. No kill of a process without a proof of ownership.

## Decisions

| ID | Decision | Reason |
|---|---|---|
| D1 | A new package `internal/proctree` holds the process table, the proofs and the reaper. The bridge and gc use it. | One kill path. Two copies of kill code can drift. |
| D2 | The table comes from `ps -A -o pid=,ppid=,pgid=,uid=,lstart=,comm=` with `LC_ALL=C`. The arguments and the environment of each pid come from `sysctl KERN_PROCARGS2` on macOS and from `/proc/<pid>/cmdline` and `/proc/<pid>/environ` on Linux. | `KERN_PROCARGS2` and `/proc` keep the arguments and the environment apart (F5). `ps -E` does not. |
| D3 | The table keeps only the rows of the current user, and only the `EVOLVE_` keys of each environment. | Least data. gc never needs other keys. |
| D4 | The identity of a process is its pid plus its start time. Each signal round lists the table again and checks the identity and the proof again. | A pid can come back as a different process. |
| D5 | Each proof is a predicate on one process (the Specification pattern). The proofs are: the dispatch tag, a recorded set, a stale dispatch tag, and an orphan log tail. | Each rule has one home and one test table. |
| D6 | The reaper sends signals to single pids only, never to a group. It refuses pid 0, pid 1, its own pid and its parent's pid. | A group signal can reach a process that left the tree. |
| D7 | The sequence is: SIGTERM, wait the grace time (2 s), list again, SIGKILL the processes that still match, list again. A process that is still there is a survivor. | Dossier R12, R14 and R15. The MCP stdio rules ask for SIGTERM before SIGKILL. |
| D8 | `tmuxCleanup` records the tree of the pane before `kill-session`: the pane pid and each descendant. The sweep then uses the tag or the recorded set. | Apple binaries hide the tag (F4). The tree is the proof for them. |
| D9 | A named session that the bridge keeps for resume gets no sweep. | Its processes stay alive for the next dispatch. |
| D10 | `driverEnv` removes an inherited `EVOLVE_DISPATCH_ID`. The dispatch then adds its own id as the last entry. The pane boot script exports the id, or unsets it when the id is empty. | A pane must not carry the tag of a different dispatch. The tmux server environment can hold a stale tag. |
| D11 | `sysexec.Command(ctx, name, args...)` is the one constructor. It sets `Cancel` to SIGTERM and `WaitDelay` to 5 s. Go sends SIGKILL when `WaitDelay` ends. A test parses each production Go file and fails on a direct `exec.Command` or `exec.CommandContext` call outside `sysexec`. | One home for the rule. The test keeps new code on it. |
| D12 | The new gc rules live in new files: `gc/dispatch_processes.go` and `cmd/evolve/cmd_gc_dispatch.go`. `gc.Process` and its cwd rule do not change. | Lane H1 edits other gc files. Separate files prevent merge conflicts. |
| D13 | A leaked test tmux session on a bridge socket is a session named `evolve-bridge-it-<tag>-<pid>`. gc kills it when the pid is dead. | Only `tmux_repl_integration_test.go` makes that name. F8 already covers the leaked test servers. |
| D15 | The tag proves descent, not ownership. A process that the dispatch started can be a shared helper that other clients now use. Thus a kill needs the tag and one more proof: membership in the recorded tree, or no membership in the shared class. The shared class is one table in `proctree/shared.go`: the program `tmux`, the Claude Code helpers (`daemon run`, `bg-pty-host`, `bg-spare`), and Chrome or Chromium. | Security review, round 1 (HIGH). |
| D16 | The bridge records the pane tree at dispatch, on each poll and before `kill-session`. The record is a union, so a child that left the tree stays a member. The bridge writes the tree to `.evolve/dispatch-trees/<id>.json` (pid and start time) when it grows. A clean sweep removes the file. | gc can apply the same membership test after the bridge is gone. A session that is gone before the cleanup still has a tree. |
| D17 | The stale-dispatch rule of gc requires a dead owner. The rule for an orphan of a closed cycle with a live owner applies only to members of the persisted tree. The gc root must be absolute. | Security review, round 1 (HIGH and L2). |
| D18 | The bridge starts each tmux command with the tag removed from its environment, so a tmux server is never tagged. The reaper never signals any ancestor of itself (the whole ppid chain from the table). | Security review, round 1 (M1 and M2). |
| D19 | Every `kill-session` site goes through one helper, `killSessionSwept`: record, then kill, then sweep. A named session that the bridge kills (a model mismatch) loses its "kept for resume" state, so its processes are swept. | Go review, round 1 (M2 and M3). |
| D20 | `sysexec.DefaultRunner` returns success with one WARN line when a command exits 0 and a child keeps the output pipe open past `WaitDelay`. | Go review, round 1 (M1). |
| D21 | A recorded shared helper is stopped only while it is a live descendant of a recorded pane at sweep time. A helper that detached (for example a `claude daemon run` that other sessions now use) is spared. gc never stops a shared helper by tree membership alone. | Security delta check, round 3 (M-a). |
| D22 | The reaper refuses to run when its own pid is not in the table, because it cannot prove its ancestors then. The tree directory is mode 0700. | Security delta check, round 3 (M-b and L-b). |
| D14 | Push-back on `TMUX_TMPDIR` for tests: the tests keep the per-pid socket in the default tmux directory. | gc lists sockets in the default directory. A private `TMUX_TMPDIR` can hide a leaked test server from the gc backstop. F8 is the root cause fix, and it is already on main. |

## The ownership proofs

| Rule | Where | Proof (all parts must be true) |
|---|---|---|
| Dispatch tag | bridge, at dispatch end | `EVOLVE_DISPATCH_ID` in the environment equals the id of this dispatch, and the process is not in the shared class. |
| Recorded tree | bridge, at dispatch end | The pid and the start time are in the recorded tree: the union of the records at dispatch, at each poll and before `kill-session`. |
| Stale dispatch | gc | The tag parses. `EVOLVE_PROJECT_ROOT` equals the gc project root, and the root is absolute. Then one of two cases. Case 1: the owner is dead, and the process is in the persisted tree or not in the shared class. The owner is the bridge pid in the tag. Case 2: the cycle of the tag is closed out, the process is an orphan, and it is in the persisted tree. |
| Orphan log tail | gc | ppid 1. The command name is `tail`. Each file argument is an absolute path under `<root>/.evolve/`. There is at least one file argument. The age is more than `gc.temp_ttl_hours`. |
| Test session | gc (`swarm`) | The session name is `evolve-bridge-it-<tag>-<pid>` on a bridge socket. The pid is dead. |

## The security model of the kill path

The kill path can stop a process that we do not own. This table lists each way, and the test that blocks it.

| # | Way to kill a foreign process | Block | Test |
|---|---|---|---|
| S1 | A foreign process carries a string that looks like our tag in its arguments (F5). | D2: the environment and the arguments are separate. The tag proof reads the environment map only. | `TestParseProcArgs2_KeepsArgumentsAndEnvironmentApart`, `TestTaggedWith_ATagInTheArgumentsIsNoProof` |
| S2 | A pid is reused by a new process between the listing and the signal. | D4: each round lists again and matches pid plus start time. | `TestReap_APidReusedByANewProcessIsNotKilled` |
| S3 | A process of a different user. | D3: the table keeps the rows of the current uid only. | `TestParsePS_KeepsOnlyTheRowsOfTheGivenUser` |
| S4 | pid 0, pid 1, the reaper itself or its parent. | D6: the guard refuses these pids. | `TestReap_NeverSignalsInitItselfOrItsParent` |
| S5 | A group signal reaches a process that left the group. | D6: per-pid signals only. | `TestReap_SignalsSinglePidsOnly` |
| S6 | Chrome, an interactive `claude` session or the Claude Code daemon. | None of them carries our tag, is in a recorded pane tree, or is a `tail` with files under `.evolve/`. | `TestStaleDispatch_RefusesEveryProcessWithoutTheProof`, `TestOrphanLogTail_Table` |
| S7 | A process that inherited `EVOLVE_PROJECT_ROOT` but no dispatch tag (an operator shell). | The stale-dispatch proof needs a tag that parses. | `TestStaleDispatch_RefusesEveryProcessWithoutTheProof` |
| S8 | A process of a live dispatch. | The owner pid in the tag is alive and the cycle is open. | `TestStaleDispatch_RefusesEveryProcessWithoutTheProof` |
| S9 | A process of a different project. | `EVOLVE_PROJECT_ROOT` must equal the gc root exactly. | `TestStaleDispatch_RefusesEveryProcessWithoutTheProof` |
| S10 | A process of a session that the bridge keeps for resume. | D9: no sweep for a named session. | `TestTmuxCleanup_ANamedSessionIsNeverSwept` |
| S11 | A child inherits the stale tag of a different dispatch from the tmux server or from the parent environment. | D10. | `TestDriverEnv_DropsAnInheritedDispatchTag`, `TestBootTmuxREPL_ExportsTheDispatchTagOfThisLaunch` |
| S12 | A `tail` that reads a file outside `.evolve/`, or a relative path. | Each file argument must be absolute and under `<root>/.evolve/`. | `TestOrphanLogTail_Table` |
| S13 | A listing fails after SIGTERM. SIGKILL can then go to pids with no proof. | The reaper stops and reports. It sends no SIGKILL without a new listing. | `TestReap_AListingFailureAfterTermSendsNoKill` |
| S14 | `--dry-run` sends a signal. | The dry run plans only. It never calls the reaper. | `TestGCDispatchProcesses_DryRunListsAndSignalsNothing` |
| S16 | A bad pane pid (0 or 1) makes the recorded tree the tree of init, which holds every orphan on the host. | `Descendants` refuses pid 0, pid 1 and negative pids as a root. | `TestDescendants_RefusesInitAndPidZeroAsARoot` |
| S17 | A leaked test session sweep kills a live test. | The test pid in the session name must be dead, and pid 0 and pid 1 are refused. | `TestReapOrphans_ALeakedTestSessionWhoseTestProcessIsDeadIsReaped` |
| S18 | A tagged shared helper: a Claude Code daemon (`daemon run`, `bg-pty-host`, `bg-spare`) or a Chrome or Chromium for a browser MCP. | D15: the tag alone is not enough for the shared class. | `TestSharedHelper_MatchesTheClassOfHelpersOthersUse`, `TestOwnedByDispatch_ATaggedSharedHelperNeedsTheRecordedTree`, `TestDispatchSweep_NeverStopsATaggedSharedHelperOutsideTheRecordedTree`, `TestStaleDispatch_NeverStopsATaggedSharedHelperOutsideTheRecordedTree` |
| S19 | A tmux server that a dispatch started, or that inherited a tag. | D18: tmux starts with no tag. D15: `tmux` is in the shared class. | `TestBoundedCommand_StartsTmuxWithoutTheDispatchTag`, the tmux rows of the shared-class tests |
| S20 | An ancestor of the reaper above its parent (the loop, a fleet supervisor, a tmux pane of a dispatch). | D18: the whole ppid chain from the table is protected. | `TestReap_NeverSignalsAnAncestorOfTheReaper` |
| S21 | A relative gc root matches a relative `EVOLVE_PROJECT_ROOT`. | D17: the root must be absolute. | `TestStaleDispatch_ARelativeRootMatchesNothing` |
| S22 | An orphan of a closed cycle whose owner is alive, but that the dispatch did not start. | D17: that case needs the persisted tree. | `TestStaleDispatch_RefusesEveryProcessWithoutTheProof` |
| S23 | A crafted dispatch id names a tree file outside the tree directory. | `TreeFile` maps every character outside `[A-Za-z0-9-]` to `_`. | `TestTreeFile_StaysInsideItsDirectory` |
| S24 | A recorded shared helper that detached from the pane and now serves other sessions. | D21. | `TestOwnedByDispatch_ARecordedSharedHelperMustStillBeALiveDescendantOfThePane`, `TestDispatchSweep_ARecordedSharedHelperIsStoppedOnlyWhileItIsALiveDescendantOfThePane`, `TestStaleDispatch_NeverStopsATaggedSharedHelperOutsideTheRecordedTree`, `TestGCDispatchProcesses_ReadsThePersistedTreeOfADeadDispatch` |
| S25 | A table without the reaper hides its ancestors. | D22. | `TestReap_RefusesToRunWhenItsOwnPidIsNotInTheTable` |
| S15 | A test kills a real process. | Unit tests use a fake table and a recording signaler. The one real test signals only the child that it started. | `go/internal/proctree/list_integration_test.go` |

## TDD protocol

1. One test at a time. Run it and see it fail on its named assertion. Keep the red output in the lane scratchpad (`red.txt`).
2. Write the smallest code that makes it pass.
3. For each fix, run a mutant of the fix and see the test fail.
4. Unit tests fake the table, the signaler and the clock. No unit test sends a signal to a real process.

## Patterns and their forces

| Pattern | Force |
|---|---|
| Specification (`proctree.Proof`) | Five ownership rules. Each needs a table test. They combine with `AnyOf`. |
| Ports and adapters (`proctree.Lister`, `proctree.Signaler`) | The process table and the signal are process boundaries. The core logic must run with fakes. |
| Template method (`Reaper.Reap`) | The TERM, wait, KILL and check sequence is the same for the bridge and for gc. Only the proof changes. |
| Not abstracted | The per-OS readers of the arguments. Each is one function behind a build tag. |

## Limits

- On macOS, some descendants have no tag and no record: an Apple binary that calls `setsid` and leaves the pane tree before the record. gc does not see it. A Linux cgroup closes this gap (N1).
- The recorded tree is a union of records at dispatch, at each poll (2 s) and before `kill-session`. A process that starts and leaves the tree between two records is in the sweep only if it carries the tag.
- The start time has a resolution of one second. A new process with the same pid in the same second as the old one can pass D4. The old process must stop and the pid must wrap in that second. We accept this risk.
- A process that ignores SIGKILL (a process in uninterruptible sleep) is a survivor. The bridge reports it and does not wait.

## Status

- [x] Plan.
- [x] `proctree`: table, readers, proofs, reaper, dispatch id.
- [x] Bridge: tag (K8) and reap at dispatch end (K9).
- [x] `sysexec.Command` and the guard test (K10): 136 calls in 78 files.
- [x] gc: stale dispatch, orphan log tails, test sessions (K11).
- [x] Docs and CHANGELOG.
- [x] Review fix round 1: D15 to D20, S18 to S23.
- [x] Final fix round: D21 and D22, S24 and S25.

## Later

The inbox item `.evolve/inbox/2026-10-08T17-00-00Z-dispatch-hygiene-followups.json` holds the first three items.

- M-c: widen the shared class: Firefox and WebKit from Playwright, Electron helpers, `ssh-agent`, `gpg-agent`. Or change the tag-only kill to an allow list of program names.
- L-a: gc removes a tree file when the owner of its dispatch is dead and all its members are gone.
- A crashed bridge can leave a tagged shared helper with no persisted tree. gc spares it, and the operator must stop it by hand.
- A Linux cgroup per dispatch (N1).
- A sweep for the other paths that kill a session without a dispatch: `capture_models.go`, `launch_model_verification.go` and `recipe_adapter.go`. They start no dispatch tag today.
- Legacy processes that have `EVOLVE_PROJECT_ROOT` but no dispatch tag (MCP servers started before this change). gc leaves them alone, by design. Do this one-time manual check after the change lands:
  1. List them: `ps -E -A -o pid=,ppid=,etime=,command= | grep EVOLVE_PROJECT_ROOT= | grep -v EVOLVE_DISPATCH_ID=`.
  2. Keep each row whose parent is alive and is a live loop, a console or an interactive `claude` session.
  3. Find each orphan (ppid 1) that a finished dispatch started, such as an MCP server (`node`, `npx` or `uvx`). Send it `kill -TERM <pid>`. Never stop `tmux`, Chrome, Chromium or a `claude daemon` process by hand from this list.
