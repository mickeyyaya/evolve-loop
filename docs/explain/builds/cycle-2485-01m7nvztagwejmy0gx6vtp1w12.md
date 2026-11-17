# Build Explanation — Cycle 2485

## Build Binding
- Cycle: 2485
- Base SHA: a1bb7cc3caa9cdea2c39e5a62471412dda64d23c

## Summary
One research note explains the Claude release notes line on linked billing as data. It gives each core persona a no-change verdict under ADR-0133.

## Rationale
Linked billing decides who pays for a request. No persona sets billing_policy, so the smallest correct change is a note and no prompt edit.

## Changed Areas
- `docs/research/practice-claude-model-release-notes-linked-billing-5cd04824-202611171847.md` — the note with six fact rows, six persona verdicts and the decision.
- `docs/research-index/2026-11-18-01-practice-claude-model-release-notes-5cd04824.md` — the index entry file for the note.
- `docs/research-index.md` — the generated index, regenerated so that it links the note.

## Design Decisions
The claude model notes stay unchanged, because linked billing is a payment fact and not a prompt behavior fact.

## Verification
The nine predicates of cycle 2485 pass, the strict sentence lint reports no finding, and the ACS suite reports no red.

## Compatibility
No persona, no notes file and no Go source changes.

## Limitations
The fetch tool cut off the last release-note line, and the live page has no cut-off entry. The note treats that incomplete line as page text and not as a fact.
