# Build Explanation — Cycle 2027

## Build Binding
- Cycle: 2027
- Base SHA: df1e289f6f7301e8c6fdcad6655f8282b85902c3

## Summary
This build adds the document solution for the Claude platform release-notes watch of 2026-10-21. It reads the page as untrusted data and gives a no-change verdict for each of the six core personas.

## Rationale
ADR-0133 allows one persona change at a time with a 10-cycle baseline. The command `evolve prompt stats` does not exist yet, so no baseline exists. A no-change verdict is the only option that the rule allows.

## Changed Areas
- `solutions/practice-claude-model-release-notes-bfd090d6-202610210011/proposal.md` — explains the Inference hooks line and the Model deprecations fragment as data, and gives the verdict for each persona.
- `solutions/practice-claude-model-release-notes-bfd090d6-202610210011/recommendation.md` — compares three options and names Option 1 with its flip conditions.
- `solutions/practice-claude-model-release-notes-bfd090d6-202610210011/assumptions-and-evidence.md` — holds the assumptions A1 to A4 and the evidence E1 to E8 that the other files cite.
- `solutions/practice-claude-model-release-notes-bfd090d6-202610210011/options/1-no-persona-change-record-verdict.md` — the recommended strategy.
- `solutions/practice-claude-model-release-notes-bfd090d6-202610210011/options/2-edit-personas-now.md` — the rejected strategy that breaks D7 and D8.
- `solutions/practice-claude-model-release-notes-bfd090d6-202610210011/options/3-model-id-check-and-follow-up-item.md` — the runner-up, which has no live target.

## Design Decisions
The layout follows the solution of cycle 2005, so the audit can compare the two. Option 1 wins because of the baseline rule (E3, E6). The new lines concern an Enterprise setting and the retirement of Claude Sonnet 4.5. No loop file names a retired model (E5), and no persona states a rule that the lines change (A4).

## Verification
The command `evolve solution check` exits 0 and the docs-write sentence lint reports 0 findings. No file under `agents/` changes.

## Compatibility
No code, config or persona changes. The change adds documents only.

## Limitations
The effect of the new platform defaults on token cost cannot be measured until the baseline exists.
