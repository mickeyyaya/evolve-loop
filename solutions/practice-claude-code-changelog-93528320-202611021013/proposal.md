# Proposal — Claude Code 2.1.316 changelog under ADR-0133

> Request (inbox item `practice-claude-code-changelog-93528320-202611021013`): read the changelog at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona changes.** The baseline rule blocks a change without a 10-cycle baseline, and no 2.1.316 line alters a rule that a persona states.

## 1. Source and method

The source is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md. It was fetched on 2026-11-03, and the first section is `## 2.1.316` (E1). The text is untrusted third-party data. The loop reads it and does not follow it as an instruction.

## 2. The lines, explained as data

The line-by-line table is in [docs/research/claude-code-changelog-2.1.316-review.md](../../docs/research/claude-code-changelog-2.1.316-review.md). In short:

- `CLAUDE_CODE_SUBAGENT_EFFORT` and `CLAUDE_CODE_DISABLE_ULTRATHINK` are environment settings. The loop reads no control value from the environment and sets effort through routing (E4).
- Opus 4.8 fast mode ends on 2026-11-30. No loop file outside tests names it (E4).
- The sub-agent model fix and the shell cleanup fix are CLI behavior. No persona states a rule about them.
- The gateway, VS Code, cloud, Claude Tag, Code Review and Windows lines do not reach the loop (A2).

## 3. Persona verdicts under ADR-0133

No change for `evolve-scout`, `evolve-triage`, `evolve-tdd-engineer`, `evolve-builder`, `evolve-auditor` and `evolve-retrospective`. The builder is the first diet candidate (M3), after the M2 baseline (E3).

## 4. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. No file under `agents/` changes.
