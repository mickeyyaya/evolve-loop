# Build Explanation — Cycle 2278

## Build Binding
- Cycle: 2278
- Base SHA: 663d563bc15ac7880a3798137077950473a54e55

## Summary
The claude model notes gain one bullet about effort on Fable 5.2 and Mythos 5.2. A research note and an index entry record the decision.

## Rationale
The guide says that an order to answer without thinking does not stop thinking at medium effort and above. The notes are the one home of model rules, so the rule goes there. No persona holds such an order, so no persona changes.

## Changed Areas
- `skills/model-notes-claude/COMPACT.md` — adds one bullet that tells agents to lower the effort level instead of ordering the model not to think.
- `docs/research/practice-claude-prompting-guide-effort-2026-11.md` — reads the guide change as untrusted data and gives a no-change verdict for each core persona under ADR-0133.
- `docs/research-index/2026-11-10-05-practice-claude-prompting-guide-effort.md` — links the research note from the index.

## Design Decisions
One bullet in the notes is the smallest change. A persona change was rejected because no persona holds a do-not-think order and ADR-0133 requires a baseline first.

## Verification
The cycle predicates pass 8 of 8, and the ACS suite passes with red=0. The sentence lint reports 0 findings on the changed files.

## Compatibility
The six existing bullets stay unchanged. The notes stay under the 12-line ceiling.

## Limitations
The item text was truncated, so only the effort claim is adopted. The generated region of the research index is not regenerated, because an earlier index entry blocks the generator.
