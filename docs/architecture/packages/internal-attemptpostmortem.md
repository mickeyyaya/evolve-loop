# internal/attemptpostmortem

> The decision: [ADR-0130](../adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md). The plan (components P1 to P5, decisions D1 to D18): [attempt-postmortem-2026-10.md](../../plans/attempt-postmortem-2026-10.md). The research: [attempt-postmortem-2026-10.md](../../research/attempt-postmortem-2026-10.md).

## Purpose

`internal/attemptpostmortem` records how one dispatch of a phase ended. It also renders the records of a phase into a section for the prompt of the next dispatch. It has these parts:

- `Record` and `Attempt`, the schema `attempt-postmortem/1.0`, with `Validate`, `Path`, `Write` and `ReadAll`;
- the suspect rule, `findSuspect`;
- the transcript port: `Transcript`, `ClaudeTranscript`, `TranscriptFor` and `ErrNoTranscript`;
- the collector, `Collect(Input, Config)`;
- the renderer, `Render(records, Config)`, with `SectionHeading`.

Nothing in production calls the package yet. The wiring lane W1 wires it after lane `cl-pane-loss` merges.

## Design

- **A leaf.** The package imports the standard library and `internal/atomicwrite` only. The cause is a string, the `cause_code` of `launchoutcome`, because `launchoutcome` imports `core`.
- **The port is a function.** `Transcript` returns a `Trace`: the path, the Bash commands in order and the last timestamp of any entry. `ClaudeTranscript` reads the `tool_use` and `tool_result` entries of a Claude Code transcript. A line that does not parse is skipped. A line over 8 MiB is an error.
- **The result status.** No `tool_result` gives `no_result`. `is_error` false gives `ok`. "Exit code N" in the first line gives `signal` above 128, else `error` with the code. Other error text gives `error` with no code.
- **The selection.** `TranscriptFor` gives the Claude adapter for `claude` or a `claude-` CLI with a path. Each other CLI, `agy-claude-tmux` included, gets `ErrNoTranscript`.
- **The fallback.** When the transcript gives an error, the record has `command_source` `pane_tail` and the error in `source_error`. It has no commands and no suspect.
- **The suspect rule.** Walk the commands from the last to the first. The first command with no result, an exit by signal, or a start at or after `end - window` is the suspect. The end is the last transcript timestamp, else `EndedAt`. The rule runs on all the commands, before the cap on their count.
- **The caps.** `capHead` keeps the start of a command or a delta and ends it with "…". `capTail` keeps the end of the pane tail and starts it with "…". The renderer caps again, because a record on disk is not trusted.
- **The section.** It has the heading, a preamble that says the quoted text is data, one block for each abnormal record in number order and two rules. A command is one code span: newlines become " ⏎ ", and the fence is one backtick longer than the longest run in the text. A block uses a fence of at least three backticks, and longer than any run in the text.
- **The write.** `Write` validates the record, then calls `atomicwrite.JSON`. `ReadAll` matches `<phase>-attempt-*-postmortem.json`, so the hidden temp files of `atomicwrite` never match. It validates each record and sorts by number, so record 10 comes after record 2.

## Invariants

- **The cycle 1853 probe is the suspect.** `TestCollect_NamesTheKillServerProbeAsTheSuspectOfCycle1853BuildAttempt1` replays the redacted fixture of the first Build session.
- **The section for the second dispatch states the probe, the cause and the rules.** `TestRender_TheSectionForBuildAttempt2StatesTheProbeTheCauseAndTheRule`.
- **A clean end gives no section.** `TestRender_ACleanEndGivesNoSection`.
- **Agent text cannot leave its span or its block.** `TestRender_AgentTextCannotBreakOutOfItsCodeSpanOrBlock` and `TestCodeSpan_KeepsAgentTextOnOneLineInsideALongerFence`.
- **The order of the rules.** `TestFindSuspect_PicksTheLastCommandThatMatchesAnyRule` pins each rule, the window edge and the precedence.

## Findings

- **The end of the dispatch is the wrong end.** In cycle 1853 the bridge ended the first dispatch 38 min 44 s after the probe. A window from that end finds nothing, so the rule uses the last sign of life of the session.
- **One anchor, two sessions.** Both Build sessions of cycle 1853 carry the same artifact anchor and the same `cwd`. The locator of W1 must add the time window of the dispatch to the attribution rule of `tokenusage`.
- **The scrollback is written over.** `tmux-final-scrollback.txt` holds the pane of the last session of the run only. The collector must run before the next session starts.
