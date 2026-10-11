# ADR-0130: A retry carries the evidence of the dispatch that it replaces

- **Status:** Accepted, wired (2026-10-10). Components P1 to P5 were built on 2026-10-09. The wiring lane W1 wired them into the runner, with the `attempt_postmortem` policy block and the verb `evolve postmortem` ([plan](../../plans/attempt-postmortem-2026-10.md) §6 and §11).
- **Plan:** [attempt-postmortem-2026-10.md](../../plans/attempt-postmortem-2026-10.md).
- **Research:** [attempt-postmortem-2026-10.md](../../research/attempt-postmortem-2026-10.md).
- **Package:** [internal-attemptpostmortem.md](../packages/internal-attemptpostmortem.md).
- **Relates to:**
  - [ADR-0072](0072-system-failure-policy-and-halt.md): the failure dossier stays the evidence of the cycle verdict;
  - [ADR-0103](0103-component-breakdown-program.md): `launchoutcome` owns the cause codes, and `failurediag` owns the sidecar of a phase abort;
  - [ADR-0111](0111-code-carries-no-comments.md): the new code has no comments.
- **Evidence:**
  - In cycle 1853, the retry of Build got the same prompt, byte for byte. Only the session name and a paste id changed (research F1.1).
  - The second session ran the same probe that ended the first session (F1.2, F1.4).
  - The bridge ended the first dispatch 38 min 44 s after the last sign of life of its session (F1.3).
  - No record of the repository holds the last commands of a dead session (F2.1).

## Context

After an abnormal end, the retry loop of the runner (`dispatchPhaseAttempts`) sends the same prompt again. An abnormal end is a lost pane, an artifact timeout, a crash or a kill. A resume does the same thing. The next agent has the same input and the same worktree, so it makes the same decision.

The escalation report says only "stalled". It does not say which command ran last, or how long before the end it ran.

## Decision

1. **One record for each abnormal end in the agent session.** The evolve runtime writes `<run>/<phase>-attempt-<n>-postmortem.json` with schema `attempt-postmortem/1.0`.
2. **The record holds the facts.** It has the identity, the times, the cause code, the exit code and the last commands with their status.
3. **More facts.** It also has the suspect command, the pane tail, the worktree delta and the evidence paths.
4. **A deterministic suspect.** The last command with no result, an exit by signal, or a start inside the window.
5. **The window ends at the last sign of life.** It is the latest transcript timestamp, else the end of the dispatch.
6. **A transcript port.** Claude gives its transcript. agy and other CLIs give the pane tail only.
7. **A section in the next prompt.** The evolve runtime states it after the cycle context, with a rule about the suspect.
8. **Agent text is quoted data.** Code spans and fences that the text cannot close, with caps.
9. **A resume reads the records.** `RunCycleFromPhase` gets the same section for the phase.
10. **A new leaf package.** `internal/attemptpostmortem`, stdlib plus `atomicwrite`. No other dossier changes.
11. **Config, not flags.** `Config` holds the caps and the window. The policy block comes with the wiring.
12. **R3 later.** A Claude `PreToolUse` hook denies a command that matches a suspect signature.

## Scope

- **In scope.** Each dispatch of `BaseRunner` (`dispatchPhaseAttempts`), a retry in its dispatch walk and a resume through `RunCycleFromPhase`.
- **No record for an end outside the agent session.** A quota wall, an exhausted account, exit 85 or 87, a boot timeout and a launch refusal do not come from the agent. A record there can name a correct command as the suspect, so the runner writes none (`endedInTheAgentSession`).
- **No record under a cancelled context.** An operator stop and the teardown of the deliverable-authority door cancel the context (`context.Canceled`), and the kill can come after a command that finished OK. The runner writes no record for them. A deadline (`context.DeadlineExceeded`, the phase or cycle timeout) keeps its record.
- **A record for exit 124.** A command timeout (exit 124) ends in the agent session, so it gets a record.
- **The fresh session is in scope (2026-10-11).** The bridge marks a `dead_shell` or `cli_self_updated` end as `SessionRecoverable`. One helper, `core.LaunchRecoverable`, runs the one fresh session. The runner's hook writes the record and adds the section (plan Q6, ADR-0044 amendment).
- **The pane tail of each dispatch (2026-10-11).** The bridge writes `<run>/<agent>-<session>-final-pane.txt` before it kills a tmux session, and names it in `core.BridgeResponse.PaneCapture`. The runner attaches it. The shared `tmux-final-scrollback.txt` names no dispatch, so the runner still does not attach it (plan Q8).
- **The dispatch id (2026-10-11).** `core.BridgeResponse.DispatchID` names the dispatch in the record (plan Q7).
- **Out of scope.** The retro phase and the swarm workers call `Launch` themselves, not through `BaseRunner`. Thus they get no record and no section. They KEEP the fresh session through `core.LaunchWithFreshSession`, with the same request.

## Alternatives considered

| Alternative | Why not |
|---|---|
| Resume the dead Claude session with `--resume` | The context is near its limit, and the model keeps the same plan. agy has no such resume. |
| An LLM reflection on the dead session | It costs a model call. It can lose the exact command, and it is not deterministic. |
| Only the repeat guard hook | The agent does not learn why a command is denied. agy has no `PreToolUse` hook. |
| Extend `failurediag` | Its sidecar is one record for each phase abort, with a fixed wire contract. A dispatch record is a different scope. |
| Extend the ADR-0072 dossier | The dossier is the evidence of the cycle verdict. A second scope makes it a second dossier. |
| A window that ends at the end of the dispatch | In cycle 1853, that window starts 38 min after the probe and finds nothing. |
| Put the section before the cycle context | It changes the cached prefix of each phase prompt. |

## Consequences

- **The next dispatch knows the earlier ones.** It sees the cause, the suspect command and the rule not to repeat it.
- **New files in the run directory.** One small JSON file for each abnormal dispatch.
- **The wiring.** The runner finds the transcript (`tokenusage.LocateTranscript`), runs `git diff --stat`, collects and writes after an abnormal end, and renders before each launch (`internal/phases/runner/postmortem.go`). A failure is a WARN, and the dispatch goes on.
- **A backfill.** `evolve postmortem collect` writes the record of a dispatch that ended before the wiring, and `evolve postmortem show` prints the section.
- **A suspect is not a cause.** The window rule can name an innocent command. The record states the reason, so a reader can judge it.
- **agy gets less.** It has no transcript, so its record has the pane tail and no command list. When no pane tail was read, `command_source` is `none`.
