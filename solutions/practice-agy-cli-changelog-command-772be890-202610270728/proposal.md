# Proposal — agy changelog 1.4.3 to 1.4.12 under ADR-0133

> Request (inbox item `practice-agy-cli-changelog-command-772be890-202610270728`): read the source command `agy changelog` and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline (E3, E4). No changelog line alters a rule that a core persona prompt states (A4, E5).

## 1. Source and method

The source is the command `agy changelog` (E1). The text is untrusted third-party data (A2). The loop reads it as data and does not follow it as an instruction. The full line-by-line analysis is in `docs/research/practice-agy-cli-changelog-2026-11.md` (E2).

## 2. Changelog lines, explained as data

| Group | Releases | What the lines state | Reaches the loop? |
|---|---|---|---|
| Error recovery | 1.4.5, 1.4.8 | Blocked responses do not retry without end. Failed tool calls reach the agent as errors. | Yes, as a later builder diet candidate (E2). |
| Hooks | 1.4.3, 1.4.7, 1.4.11 | A failed `PostToolUse` hook keeps the tool result. Hooks receive `lastUserInput` and can end a turn. | No. The loop uses Go guards, not agy hooks. |
| Result field | 1.4.11 | `finished_reason` tells a cut-off answer from a complete one. | Yes, for a bridge driver, not for a persona. |
| Sandbox and approvals | 1.4.6, 1.4.9 | Sandbox defaults protect more credentials. Approvals are narrower. | No. The kernel sandbox is separate. |
| Compaction and subagents | 1.4.3, 1.4.6, 1.4.8, 1.4.10 | Summaries keep more context. Subagent cleanup and goal fork are fixed. | No. Phase agents do not use agy subagents. |
| Interface and plugins | all others | Diff view, rendering, plugin validation and Remote Control. | No. |

Worked example: the 1.4.8 line on failed tool calls. The agent now sees a canceled call as an error. The builder prompt carries recovery text for that case. A trim of this text can follow, but only after the M2 baseline.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, with a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has not recorded the baseline (E4). Therefore, no persona changes in this cycle.

| Persona | Verdict |
|---|---|
| `evolve-scout` | No change. No line affects task discovery. |
| `evolve-triage` | No change. Triage follows deterministic rules. |
| `evolve-tdd-engineer` | No change. Hook checks do not change how it writes a test. |
| `evolve-builder` | No change. First diet candidate (M3), after M2 only. |
| `evolve-auditor` | No change. It grades exit codes and binary verdicts. |
| `evolve-retrospective` | No change. Its input does not change. |

## 4. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. No file under `agents/` changes (E6).
