# ADR-0130: A retry carries the evidence of the dispatch that it replaces

- **Status:** Proposed (2026-10-09). Components P1 to P5 are built and not wired. The decision moves to Accepted when the wiring lane W1 lands ([plan](../../plans/attempt-postmortem-2026-10.md) §11).
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

1. **One record for each abnormal end.** The bridge writes `<run>/<phase>-attempt-<n>-postmortem.json` with schema `attempt-postmortem/1.0`.
2. **The record holds the facts.** It has the identity, the times, the cause code, the exit code and the last commands with their status.
3. **More facts.** It also has the suspect command, the pane tail, the worktree delta and the evidence paths.
4. **A deterministic suspect.** The last command with no result, an exit by signal, or a start inside the window.
5. **The window ends at the last sign of life.** It is the latest transcript timestamp, else the end of the dispatch.
6. **A transcript port.** Claude gives its transcript. agy and other CLIs give the pane tail only.
7. **A section in the next prompt.** The bridge states it after the cycle context, with a do-not-repeat rule.
8. **Agent text is quoted data.** Code spans and fences that the text cannot close, with caps.
9. **A resume reads the records.** `RunCycleFromPhase` gets the same section for the phase.
10. **A new leaf package.** `internal/attemptpostmortem`, stdlib plus `atomicwrite`. No other dossier changes.
11. **Config, not flags.** `Config` holds the caps and the window. The policy block comes with the wiring.
12. **R3 later.** A Claude `PreToolUse` hook denies a command that matches a suspect signature.

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
- **New work for the wiring lane.** W1 finds the transcript, runs `git diff --stat`, collects, writes and renders. It waits for `cl-pane-loss`, which owns `dispatch.go` and the cause `pane_lost`.
- **A suspect is not a cause.** The window rule can name an innocent command. The record states the reason, so a reader can judge it.
- **agy gets less.** It has no transcript, so its record has the pane tail and no command list.
