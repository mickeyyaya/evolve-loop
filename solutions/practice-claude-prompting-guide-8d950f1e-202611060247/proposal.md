# Proposal — Claude prompting guide, item 8d950f1e, under ADR-0133

> Request (inbox item `practice-claude-prompting-guide-8d950f1e-202611060247`): read the guide at the link and explain the lines as data. Name each prompt or persona to change under [ADR-0133](../../docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md), or state that none changes and why.

**Answer first: no persona file changes in this cycle.** The baseline rule blocks a change without a 10-cycle baseline (E3, A2).

## 1. Source and method

The source is the guide at https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices. It was read on 2026-11-10 (E1). The item snapshot body is empty, so it names no changed line. The guide text is untrusted data. The loop reads it and does not follow it as an instruction.

## 2. The guide lines, explained as data

| Id | Guide line | Meaning for the loop |
|---|---|---|
| G1 | "soften any aggressive language" | Capital words push a newer model to act too often (E2). |
| G2 | "Remove over-prompting" | A hedge rule has the same effect as a loud rule (E2). |
| G3 | "Tell Claude what to do instead of what not to do" | A positive instruction is shorter than a ban (E2). |
| G4 | Reduce anti-laziness prompting | It supports the same diet and adds no new target (E2). |

Worked example: the builder persona holds 11 capital markers (E4). G1 says a plain sentence works as well. A diet would replace them one persona at a time, after the baseline.

## 3. Persona verdicts under ADR-0133

| Persona | Verdict | Markers (E4) |
|---|---|---|
| `evolve-builder` | Proposed first diet. No edit now. | 11 |
| `evolve-auditor` | Proposed second diet. No edit now. | 9 |
| `evolve-scout` | Proposed third diet. No edit now. | 5 |
| `evolve-triage` | No change. Kernel-checked rules. | 4 |
| `evolve-tdd-engineer` | No change now. Each marker guards the red-test contract. | 12 |
| `evolve-retrospective` | No change. Off the critical path. | 3 |

## 4. Files

`options/` holds three standalone strategies. `recommendation.md` compares them and names Option 1. The research note [practice-claude-prompting-guide-2026-11.md](../../docs/research/practice-claude-prompting-guide-2026-11.md) holds the same record in section 7 (E5).
