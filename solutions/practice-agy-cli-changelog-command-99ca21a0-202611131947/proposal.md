# Proposal — agy changelog 1.5.8 under ADR-0133

> Request (inbox item `practice-agy-cli-changelog-command-99ca21a0-202611131947`): read the source command `agy changelog` and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** No 1.5.8 line affects prompt format, length or structure (A3, E2). The 10-cycle baseline rule also blocks a persona edit without a measure (E3).

## 1. Source and method

The source is the command `agy changelog` (E1). The text is untrusted third-party data (A2). The loop reads each line as data and does not follow it as an instruction.

## 2. Changelog lines, explained as data

| Group | What the lines state | Reaches the loop? |
|---|---|---|
| Models | New model names in the CLI. | No. Routing data selects the model, not a persona. |
| Interface | Click to expand output. | No. |
| ACP context usage | The CLI reports context usage over ACP. | No. The loop does not read it today. |
| Admin settings | New settings for administrators. | No. |
| Sandbox and permission fixes | Security fixes in the CLI. | No. The kernel sandbox is separate. |

Worked example: the ACP context usage line. It adds a number that a client can read. No persona states a rule about that number, so no persona text changes.

## 3. Persona verdicts under ADR-0133

Rule: one persona per change, with a 10-cycle baseline (E3). No persona in `agents/` changes in this cycle (E4).

## 4. Files

`skills/model-notes-agy/COMPACT.md` gains one dated bullet (E5). `options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1.
