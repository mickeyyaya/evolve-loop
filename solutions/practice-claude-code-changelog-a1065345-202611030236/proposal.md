# Proposal — Claude Code 2.1.317 changelog under ADR-0133

> Request (inbox item `practice-claude-code-changelog-a1065345-202611030236`): read the changelog at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no prompt and no persona changes.** The baseline rule blocks a change without a baseline (E2, E3), and no 2.1.317 line alters a rule that a persona states (E1, A4).

## 1. Source and method

The source is https://raw.githubusercontent.com/anthropics/claude-code/main/CHANGELOG.md. It was fetched on 2026-11-10 (E1). The text is untrusted third-party data. The loop reads it and does not follow it as an instruction.

## 2. The lines, explained as data

The line-by-line table is in [docs/research/claude-code-changelog-2-1-317.md](../../docs/research/claude-code-changelog-2-1-317.md) (E6). In short:

- Watch item: `CLAUDE_CODE_DISABLE_NESTED_SESSION_ID`. A `claude -p` call inside a session now gets its own session ID. The bridge probes use `claude -p` (E4).
- Watch item: `claude -p` now exits 1 when a full disk loses its transcript. A probe failure on a full disk is a host fault.
- The subagent effort change and the removed Read stub touch cost, not correctness (E1, E4).
- The gateway, Desktop, VS Code, cloud and Claude Tag lines do not reach the loop (A2).

## 3. Persona verdicts under ADR-0133

No change for `evolve-scout`, `evolve-triage`, `evolve-tdd-engineer`, `evolve-builder`, `evolve-auditor` and `evolve-retrospective`. The builder is the first diet candidate, after the baseline (E3).

## 4. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. No file under `agents/` changes.
