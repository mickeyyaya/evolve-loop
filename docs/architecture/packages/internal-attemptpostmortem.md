# internal/attemptpostmortem

> The decision: [ADR-0130](../adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md). The plan (components P1 to P5, decisions D1 to D18): [attempt-postmortem-2026-10.md](../../plans/attempt-postmortem-2026-10.md). The research: [attempt-postmortem-2026-10.md](../../research/attempt-postmortem-2026-10.md).

## Purpose

`internal/attemptpostmortem` records how one dispatch of a phase ended. It also renders the records of a phase into a section for the prompt of the next dispatch. It has these parts:

- `Record` and `Attempt`, the schema `attempt-postmortem/1.0`, with `Validate`, `ValidPhase`, `Path`, `NextNumber`, `Write` and `ReadAll`;
- the suspect rule, `findSuspect`;
- the transcript port: `Transcript`, `ClaudeTranscript`, `TranscriptFor`, `WritesTranscript` and `ErrNoTranscript`;
- the collector, `Collect(Input, Config)`;
- the renderer, `Render(records, Config)`, with `SectionHeading`.

Two callers use the package (lane W1, 2026-10-10):

- the phase runner (`internal/phases/runner/postmortem.go`) renders the section before each launch and collects a record after each abnormal end ([internal-phases-runner.md](internal-phases-runner.md));
- the operator verb `evolve postmortem collect|show` ([cmd-evolve.md](cmd-evolve.md)) writes a record for a dispatch that ended before the wiring, and prints the section.

## Design

- **A leaf.** The package imports the standard library and `internal/atomicwrite` only. The cause is a string, the `cause_code` of `launchoutcome`, because `launchoutcome` imports `core`.
- **The port is a function.** `Transcript` returns a `Trace`: the path, the Bash commands in order and the last timestamp of any entry. `ClaudeTranscript` reads the `tool_use` and `tool_result` entries of a Claude Code transcript.

  A line that does not parse is skipped. A line over 8 MiB is also skipped, and the reader keeps the other lines. The count of skipped lines is in `Trace.SkippedLines` and in the record field `skipped_transcript_lines`. Only a read error of the file is an error.
- **The result status.** No `tool_result` gives `no_result`. `is_error` false gives `ok`. "Exit code N" in the first line gives `signal` above 128, else `error` with the code. Other error text gives `error` with no code.
- **The selection.** `WritesTranscript` is true for `claude` and each `claude-` CLI. `TranscriptFor` gives the Claude adapter for such a CLI with a path. Each other CLI, `agy-claude-tmux` included, gets `ErrNoTranscript`.
- **The fallback.** When the transcript gives an error, or `Input.Transcript` is nil, the record has `command_source` `pane_tail` and the error in `source_error`. It has no commands and no suspect.
- **The suspect rule.** Walk the commands from the last to the first. The first command with no result, an exit by signal, or a start at or after `end - window` is the suspect. The end is the last transcript timestamp, else `EndedAt`. The rule runs on all the commands, before the cap on their count.
- **The caps.** `capHead` keeps the start of a command or a delta and ends it with "…". `capTail` keeps the end of the pane tail and starts it with "…". The renderer caps again, because a record on disk is not trusted. It keeps the last `MaxRecords` abnormal records and the last `MaxCommands` commands of each record. It caps the CLI, the session, the dispatch id and the cause code at 120 runes, and it writes `unknown` for an empty field.
- **Control characters.** The collector and the renderer remove each C0 control character from the pane tail and the delta. They keep the newline and the tab.
- **The phase is a bare name.** `ValidPhase` accepts a letter or digit, then letters, digits, `_` and `-`. `Validate` and `ReadAll` refuse each other phase, because the phase is part of a file name and of a glob pattern.
- **The time before the end.** The section clamps a negative "before the end" time to 0 s.
- **The next number.** `NextNumber` gives the first number with no record file of the phase. The runner uses it, so a phase that runs again in the same cycle does not write over an earlier record.
- **The section.** It has the heading, a preamble that says the quoted text is data, one block for each abnormal record in number order and two rules. A command is one code span: newlines become " ⏎ ", and the fence is one backtick longer than the longest run in the text. A block uses a fence of at least three backticks, and longer than any run in the text.
- **The write.** `Write` validates the record, then calls `atomicwrite.JSON`. `ReadAll` matches `<phase>-attempt-*-postmortem.json`, so the hidden temp files of `atomicwrite` never match. It validates each record and sorts by number, so record 10 comes after record 2.

## Invariants

- **The cycle 1853 probe is the suspect.** `TestCollect_NamesTheKillServerProbeAsTheSuspectOfCycle1853BuildAttempt1` replays the redacted fixture of the first Build session.
- **The section for the second dispatch states the probe, the cause and the rules.** `TestRender_TheSectionForBuildAttempt2StatesTheProbeTheCauseAndTheRule`.
- **A clean end gives no section.** `TestRender_ACleanEndGivesNoSection`.
- **Agent text cannot leave its span or its block.** `TestRender_AgentTextCannotBreakOutOfItsCodeSpanOrBlock` and `TestCodeSpan_KeepsAgentTextOnOneLineInsideALongerFence`.
- **The order of the rules.** `TestFindSuspect_PicksTheLastCommandThatMatchesAnyRule` pins each rule, the window edge and the precedence.
- **The R2 review hardening.** `hardening_test.go` has one test for each item. The items are the bare phase, a nil transcript, the render caps, the skipped long line, the clamp to 0 s and the C0 strip. It also pins `MaxRecords`, `NextNumber`, `WritesTranscript` and `unknown` for an empty field.

## Findings

- **The end of the dispatch is the wrong end.** In cycle 1853 the bridge ended the first dispatch 38 min 44 s after the probe. A window from that end finds nothing, so the rule uses the last sign of life of the session.
- **One anchor, two sessions.** Both Build sessions of cycle 1853 carry the same artifact anchor and the same `cwd`. The locator `tokenusage.LocateTranscript` adds the time window of the dispatch to the attribution rule of `tokenusage`.
- **The scrollback names no dispatch.** `tmux-final-scrollback.txt` is one file for the whole run, and the bridge writes it only when the session is still alive. The runner thus does not attach it. `ScrollbackPath` stays for a caller that can prove the source.
