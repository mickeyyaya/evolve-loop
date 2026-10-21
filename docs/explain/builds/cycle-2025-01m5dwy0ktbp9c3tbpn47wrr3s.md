# Build Explanation — Cycle 2025

## Build Binding
- Cycle: 2025
- Base SHA: 9aa8399ad9caf2d778e52ee42ba52c9c4711df2f

## Summary
This build adds the document solution for the Claude Code 2.1.305 changelog watch. It reads the changelog as untrusted data and gives a no-change verdict for each of the six core personas.

## Rationale
ADR-0133 allows one persona change at a time with a 10-cycle baseline. The command `evolve prompt stats` does not exist yet, so no baseline exists. A no-change verdict is the only option that the rule allows.

## Changed Areas
- `solutions/practice-claude-code-changelog-218b6f25-202610210011/proposal.md` — groups the 2.1.305 lines as data and gives the verdict for each persona.
- `solutions/practice-claude-code-changelog-218b6f25-202610210011/recommendation.md` — compares three options and names Option 1 with its flip conditions.
- `solutions/practice-claude-code-changelog-218b6f25-202610210011/assumptions-and-evidence.md` — holds the assumptions A1 to A4 and the evidence E1 to E9 that the other files cite.
- `solutions/practice-claude-code-changelog-218b6f25-202610210011/options/1-no-persona-change-record-verdict.md` — the recommended strategy.
- `solutions/practice-claude-code-changelog-218b6f25-202610210011/options/2-edit-personas-now.md` — the rejected strategy that breaks D7 and D8.
- `solutions/practice-claude-code-changelog-218b6f25-202610210011/options/3-deterministic-guard-and-follow-up-item.md` — the runner-up, which has no live target.

## Design Decisions
The layout follows the 2.1.304 solution, so the audit can compare the two. Option 1 wins because of the baseline rule (E3, E4). ADR-0133 allows one persona change with a measured baseline. The command `evolve prompt stats` does not exist yet, so no baseline exists. The runtime fixes of 2.1.305 belong to the CLI, and no persona states a rule they change (E5, A4).

## Verification
The command `evolve solution check` exits 0 and the docs-write sentence lint reports 0 findings. No file under `agents/` changes.

## Compatibility
No code, config or persona changes. The change adds documents only.

## Limitations
The effect of the 2.1.305 fixes on ship rate cannot be measured until the baseline exists.
