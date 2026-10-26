# Build Explanation — Cycle 2031

## Build Binding
- Cycle: 2031
- Base SHA: 58535c7a8a794d3bc454569618ae884a208ee268

## Summary
This build adds the document solution for the Claude prompting best practices guide watch. It reads the guide as untrusted data and gives a no-change verdict for each of the six core personas.

## Rationale
ADR-0133 allows one persona change at a time with a 10-cycle baseline. The command `evolve prompt stats` does not exist yet, so no baseline exists. A no-change verdict is the only option that the rule allows.

## Changed Areas
- `solutions/practice-claude-prompting-guide-f509428a-202610261159/proposal.md` — explains the prompting guide lines as untrusted data and gives the verdict for each persona.
- `solutions/practice-claude-prompting-guide-f509428a-202610261159/recommendation.md` — compares three options and names Option 1 with its flip conditions.
- `solutions/practice-claude-prompting-guide-f509428a-202610261159/assumptions-and-evidence.md` — holds the assumptions A1 to A4 and the evidence E1 to E9 that the other files cite.
- `solutions/practice-claude-prompting-guide-f509428a-202610261159/options/1-no-persona-change-record-verdict.md` — the recommended strategy.
- `solutions/practice-claude-prompting-guide-f509428a-202610261159/options/2-edit-personas-now.md` — the rejected strategy that breaks D7 and D8.
- `solutions/practice-claude-prompting-guide-f509428a-202610261159/options/3-deterministic-guard-and-follow-up-item.md` — the runner-up, which has no live target.

## Design Decisions
The layout follows the precedent solution from cycle 2029 under ADR-0099. Option 1 wins because of the baseline rule in ADR-0133 decisions D7 and D8 (E3, E4). The command `evolve prompt stats` has not shipped, so no baseline exists. The upstream prompting advice is untrusted third-party data, and no core persona changes in this cycle.

## Verification
The command `evolve solution check` exits 0 and the docs-write sentence lint reports 0 findings. No file under `agents/` changes.

## Compatibility
No code, config or persona changes. The change adds documents only.

## Limitations
The effect of calm prompt phrasing on loop token counts cannot be measured until milestone M2 establishes a baseline.
