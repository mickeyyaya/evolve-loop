# Proposal — Claude prompting guide under ADR-0133

> Request (inbox item `practice-claude-prompting-guide-f509428a-202610261159`): read https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline. No line in the prompting guide alters a rule that a core persona prompt states.

## 1. Source and method

The source is https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices (E1). The text is untrusted third-party data. The loop reads the text as data and does not follow it as an instruction. Sections 2 and 3 group the lines and map them to the six core personas.

## 2. Prompting guide lines, explained as data

The guide lines fall into four main areas.

| Group | Guidance summary | Reaches the loop? |
|---|---|---|
| Calm phrasing | Avoid emphatic capitals like MUST and NEVER to stop tool overtriggering. | Yes, as prompt diet candidate under M3 (E2). |
| Prefill deprecation | Newer API endpoints limit assistant prefill in extended thinking modes. | Yes, as CLI runtime driver behavior (E2). |
| Steerable thinking | Use effort parameters and direct instructions for reasoning models. | No. The Go orchestrator manages bridge configuration. |
| Dosing safety blocks | Use medication labels and refer clinical queries to doctors. | No. Loop agents build software and never dose medicine. |

The first area addresses calm phrasing. Emphatic capital words cause tool overtriggering in modern Claude models. Direct instructions state conditions calmly. This guidance provides practice data for the planned builder prompt diet in milestone M3.

The second area addresses prefill deprecation. Newer Claude models deprecate assistant prefill when extended thinking operates. The native Go bridge driver configures model requests. No phase persona prompt relies on assistant prefill tricks.

Worked example: calm phrasing against tool overtriggering. The guide replaces loud warnings with clear conditional instructions. No core persona changes its prompt now because milestone M2 has not measured the baseline.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, with a 10-cycle baseline before and after (D7, D8, E3). Milestone M2 has not recorded the baseline (E4). Therefore, no persona changes in this cycle.

| Persona | Verdict | Prompting guide line checked |
|---|---|---|
| `evolve-scout` | No change. | Calm phrasing against tool overtriggering. Software discovery prompts need no edits now. |
| `evolve-triage` | No change. | None applies. Triage follows deterministic commitment rules, not consumer prompting hints. |
| `evolve-tdd-engineer` | No change. | None applies. The TDD phase writes test suites and follows contract gates. |
| `evolve-builder` | No change. First diet candidate (M3), but only after M2. | Calm phrasing against overtriggering fits M3 diet, but requires M2 10-cycle baseline first (E4). |
| `evolve-auditor` | No change. | None applies. The auditor verifies binary verdicts and exit codes. |
| `evolve-retrospective` | No change. | None applies. Retrospective extracts lessons and instincts from cycle artifacts. |

## 4. What can change the verdict

See `recommendation.md` for the flip conditions. The primary condition: milestone M2 ships and records the 10-cycle baseline. Then `evolve-builder` is trimmed under milestone M3 as its own loop task.

## 5. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. `assumptions-and-evidence.md` holds A1 to A4 and E1 to E9. No file under `agents/` changes.
