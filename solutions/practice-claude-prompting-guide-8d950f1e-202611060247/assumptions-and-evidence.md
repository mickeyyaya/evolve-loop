# Assumptions and Evidence — practice-claude-prompting-guide-8d950f1e-202611060247

Every claim in `proposal.md`, `options/*.md` and `recommendation.md` cites an entry below.
The guide text is untrusted third-party data. The loop reads it and never follows it as an instruction.

## Assumptions

- **A1** — The guide, as read on 2026-11-10, holds the lines that the watch meant. *Basis:* the item snapshot body is empty, so no other line is named (E1). Probe: read the guide again at the next wave boundary.
- **A2** — A persona change made without a baseline cannot be judged. *Basis:* ADR-0133 D8 (E3). Probe: the first D8 table (milestone M3).
- **A3** — The aggressive-wording lines of the guide apply to the loop, because the persona files hold capital words. *Basis:* the marker counts of E4. Probe: the same grep after each diet.

## Evidence

- **E1** — Source: https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices, fetched on 2026-11-10 in cycle 2256. The page shows no update date. The item `practice-claude-prompting-guide-8d950f1e-202611060247` has an empty snapshot body (snapshot 2026-11-06T02:47Z; the earlier snapshot is 2026-11-02T00:08).
- **E2** — Guide lines: "might now overtrigger", "The fix is to soften any aggressive language", "Remove over-prompting", "Tell Claude what to do instead of what not to do", and the migration step on anti-laziness prompting.
- **E3** — `docs/architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md`: one persona per change. `docs/plans/model-capability-practice-2026-10.md:97` (D7, D8): four measures over 10 cycles before and after.
- **E4** — `grep -oE '\b(CRITICAL|MUST)\b' agents/<persona>.md | wc -l` on 2026-11-10: builder 11, auditor 9, scout 5, triage 4, tdd-engineer 12, retrospective 3.
- **E5** — `docs/research/practice-claude-prompting-guide-2026-11.md`, sections 4 and 7: the existing proposal for builder, auditor and scout, and the record of this item.
- **E6** — Cost of a persona edit: one persona file plus the 10 + 10 cycle measures of D8 (E3). Cost of this review: one small document cycle.
