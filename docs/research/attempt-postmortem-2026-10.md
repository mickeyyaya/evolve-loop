# Retry context for agent harnesses: research dossier (2026-10)

> The plan that uses this research: [attempt-postmortem-2026-10.md](../plans/attempt-postmortem-2026-10.md). The decision record: [ADR-0130](../architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md). The package notes: [internal-attemptpostmortem.md](../architecture/packages/internal-attemptpostmortem.md).
>
> The refinements R1 to R9 at the end map to the decisions of the plan.

Date: 2026-10-09. Method: I read the cycle 1853 evidence on disk and the code that writes it. I checked each external claim against its primary source, and each claim has its URL. "Synthesis" marks my own inference.

## Table of contents

1. [Local evidence: cycle 1853](#1-local-evidence-cycle-1853)
2. [What the repository records today](#2-what-the-repository-records-today)
3. [Prior art](#3-prior-art)
4. [Adopt or reject](#4-adopt-or-reject)
5. [Refinements to adopt](#5-refinements-to-adopt)

## 1. Local evidence: cycle 1853

**F1.1** The two Build prompts of cycle 1853 differ only in the session name and the `pasted_content` id. Source: `diff` of the two prompts in the console investigation (root cause, step 6).

*Implication:* the retry had no fact about the first session.

**F1.2** The first Build session (`n6`, transcript `c61dea20…`) ran its last Bash command at 09:21:08.008Z. The command was the probe `TMUX_TMPDIR=$d tmux -f /dev/null new-session -d -s probe; …; TMUX_TMPDIR=$d tmux kill-server`. Its tool result came at 09:21:08.277Z with `rc=0`, and the transcript has no later entry.

*Implication:* the last sign of life of the session is 0.27 s after the start of the probe.

**F1.3** The bridge ended that dispatch at 09:59:52Z with exit code 81 and the cause code `review_pause`. Source: `llm-calls.ndjson` of cycle 1853.

*Implication:* the end of the dispatch is 38 min 44 s after the end of the session. A window that starts at the bridge end cannot see the probe.

**F1.4** The second Build session (`nb`, transcript `86c13298…`) ran the same probe at 10:02:17.581Z. Its tool result says "Exit code 137" (a SIGKILL). The bridge ended the dispatch at 10:41:21Z, again with exit code 81 and `review_pause`.

*Implication:* same input, same worktree state and the same decision: a deterministic replay.

**F1.5** `build-escalation-report.json` says only "no output during the last 1200s interval — stalled". Its `final_pane` is "no server running on /private/tmp/tmux-501/evolve-bridge-p21421".

*Implication:* the escalation report names the symptom, not the command.

**F1.6** Both Build transcripts have the same anchor in the first user message: `<artifact-path>…/cycle-1853/build-report.md`. They also have the same `cwd`.

*Implication:* the attribution rule of `tokenusage` finds both sessions. Only the time window of the dispatch separates them.

## 2. What the repository records today

| Record | Writer | Scope | Has the last commands? |
|---|---|---|---|
| `<phase>-failure-diag.json` | `internal/core/failurediag` | one for each phase abort: the error text, the exit code, the count of retries | no |
| `failure-dossier.json` (ADR-0072) | `internal/core` (`writeFailureDossier`) | one for each cycle: the verdict, the audit class, the counters of non-progress | no |
| `state.json:failedApproaches` | `internal/failureadapter` reads it | one for each failed cycle: the classification for the adaptation kernel | no |
| `abnormal-events.jsonl` | `internal/core` (cycle epilogue) | cycle events; cycle 1853 has one line, `phase-outputs-surveyed` | no |
| `tmux-final-scrollback.txt` | `internal/bridge` (tmux drivers) | the pane of the last tmux session of the run; each session writes over it | no; it holds pane text only |
| `<phase>-escalation-report.json`, `escalation-report.json` | `internal/bridge` | the pattern, the action and a pane tail | no |
| `tmux-sessions.jsonl` | `internal/sessionrecord` | the session name, the agent and the creation time | no |
| `llm-calls.ndjson` | `internal/bridge` | one for each dispatch: start, end, exit code and `cause_code` | no |

**F2.1** No record holds the last commands of a dead session. Each record names the end or the symptom.

**F2.2** `tmux-final-scrollback.txt` of cycle 1853 holds the pane of the agy session `ne` at 10:42Z, not the pane of `n6` or `nb`. Source: the file, and `driver_tmux_prepare.go:68`.

*Implication:* a postmortem must read the scrollback right after the dispatch ends, before the next session writes over it.

**F2.3** `tokenusage.ScanConfigRoot` locates a Claude transcript. It walks `<config root>/projects/**/*.jsonl` and attributes a file by the artifact-path anchor in the first user message, or else by an exact `cwd`. Source: `go/internal/tokenusage/scanner.go`.

*Implication:* reuse this rule to find the transcript path. Add the dispatch time window, because of F1.6.

**F2.4** `launchoutcome.Classify` gives the `cause_code` of an exit: the class name, or on exit 81 the typed sub-cause. Source: `go/internal/bridge/launchoutcome/classify.go`.

*Implication:* the postmortem takes the cause code as a string. It does not import the bridge.

## 3. Prior art

**F3.1 Reflexion.** The agent writes a verbal reflection on a failed trial and keeps it for the next trial. The abstract says that agents "maintain their own reflective text in an episodic memory buffer to induce better decision-making in subsequent trials". Source: Shinn and others, [arXiv:2303.11366](https://arxiv.org/abs/2303.11366).

*Implication:* the next trial gets text about the failed trial. In Reflexion, the agent writes that text. Here the agent is dead, so the bridge must write it.

**F3.2 SWE-agent.** The edit command of the agent-computer interface runs a linter. "Invalid edits are discarded, and the agent is asked to try editing the file again" (§3). When a command gives no output, the interface says "Your command ran successfully and did not produce any output". Source: Yang and others, [arXiv:2405.15793](https://arxiv.org/html/2405.15793).

*Implication:* the harness, not the model, states the facts of a failure in a fixed form.

**F3.3 OpenHands condenser.** The `LLMSummarizingCondenser` keeps the first and the last events and replaces the middle events with a summary. The summary keeps the goals of the user, the progress and the work that remains. Source: [OpenHands blog](https://openhands.dev/blog/openhands-context-condensensation-for-more-efficient-ai-agents) and [SDK condenser docs](https://docs.openhands.dev/sdk/arch/condenser).

*Implication:* keep the last events whole. A summary of the middle is not necessary for a short record.

**F3.4 OpenHands stuck detector.** The detector finds five loop patterns. Two are the same action with the same observation four or more times, and the same action with an error three or more times. It can halt the run. Source: [OpenHands agent stuck detector](https://docs.openhands.dev/sdk/guides/agent-stuck-detector).

*Implication:* a loop guard inside one session exists. It does not see a loop across two sessions, because the second session starts empty.

**F3.5 Claude Code `--resume` and `--continue`.** `--continue` loads the most recent conversation in the directory. `--resume` resumes a session by id, by name or by the path of its `.jsonl` transcript. Source: [Claude Code CLI reference](https://code.claude.com/docs/en/cli-reference).

*Implication:* a resume of the dead session gives the model its full history, with the probe. It also gives a context that is near its limit, and the same state of mind. Synthesis: a new session with a short, bridge-stated record is safer.

**F3.6 Claude Code PreToolUse hooks.** A `PreToolUse` hook can deny a tool call. It prints `permissionDecision: "deny"` with a `permissionDecisionReason`, or it exits with code 2. Source: [Claude Code hooks](https://code.claude.com/docs/en/hooks).

*Implication:* a repeat guard (R3) can deny a command whose signature matches a suspect command.

**F3.7 Aider.** When the lint or test command returns a non-zero exit code, Aider takes its output and tries to repair the code. Source: [Aider lint and test docs](https://aider.chat/docs/usage/lint-test.html).

*Implication:* the error text goes back to the model inside the same session. This does not cover a dead session.

**F3.8 LangGraph checkpoints.** A checkpointer keeps snapshots of the graph state. If a node fails in a super-step, LangGraph keeps the pending writes of the nodes that completed. A resume does not run those nodes again. Source: [LangGraph persistence](https://docs.langchain.com/oss/python/langgraph/persistence) and [LangGraph checkpointers](https://docs.langchain.com/oss/python/langgraph/checkpointers).

*Implication:* a resume reads stored records. The postmortem files are the stored records of a phase, and `RunCycleFromPhase` must read them.

**F3.9 Retry of the same input.** The AWS Builders' Library says that "client errors should not be retried with the same request because they aren't going to succeed later". Source: [Timeouts, retries and backoff with jitter](https://builder.aws.com/content/3EumjoZascWd1oZiEgL8ORlv3qE/timeouts-retries-and-backoff-with-jitter).

*Implication:* a retry with the same prompt and the same worktree is a retry of the same request. Synthesis: for an LLM agent, it is a replay of the same decision (F1.4).

**F3.10 Multi-agent failure taxonomy.** MAST lists 14 failure modes. Two of them are FM-1.3 "Step repetition" and FM-1.4 "Loss of conversation history". Source: Cemri and others, [arXiv:2503.13657](https://arxiv.org/html/2503.13657).

*Implication:* cycle 1853 shows both modes at once: the history was lost, and the step repeated.

## 4. Adopt or reject

| Idea | Decision | Reason |
|---|---|---|
| A record of each failed dispatch, given to the next one (F3.1) | adopt | It is the only channel between two sessions. |
| The harness states the facts in a fixed form (F3.2) | adopt | The dead agent cannot write a reflection. A deterministic record cannot be wrong about what ran. |
| An LLM summary of the dead session (F3.3) | reject | The bridge needs only the last commands. A summary costs a model call and can lose the command text. |
| A loop guard inside one session (F3.4) | reject for now | The repeat crosses sessions. R3 is the cross-session form. |
| Resume of the dead session (F3.5) | reject | The context is near its limit, and the model keeps the same plan. |
| A deny hook on a repeated command (F3.6) | adopt later (R3) | It is defense in depth for Claude. agy has no such hook. |
| Stored records read by a resume (F3.8) | adopt | A resumed cycle must know the earlier dispatches too. |

## 5. Refinements to adopt

- **R1** The bridge writes one record for each abnormal end of a dispatch, with schema `attempt-postmortem/1.0` (F1.5, F2.1).
- **R2** The record holds the last commands from the Claude transcript, with the start time and the result status of each (F1.2, F1.4).
- **R3** The suspect rule uses the last sign of life of the session, not the end of the dispatch (F1.3).
- **R4** The rule is deterministic: no result, an exit by signal, or a start inside a window before the end (F3.2).
- **R5** The transcript locator reuses the attribution rule of `tokenusage` and adds the time window of the dispatch (F1.6, F2.3).
- **R6** The collector reads `tmux-final-scrollback.txt` right after the dispatch ends (F2.2).
- **R7** The section in the next prompt is bridge-stated. The agent text in it is quoted data with caps (F3.2).
- **R8** `RunCycleFromPhase` reads the stored records of the phase (F3.8).
- **R9** A Claude `PreToolUse` hook denies a command that matches a suspect signature. Design it now, and build it in a later lane (F3.6).
