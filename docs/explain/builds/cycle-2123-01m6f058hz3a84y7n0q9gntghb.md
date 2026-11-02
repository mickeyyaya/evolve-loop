# Build Explanation — Cycle 2123

## Build Binding
- Cycle: 2123
- Base SHA: c28f2b67d90f65fa1a0f46f32b71261a87d44a58

## Summary
The build adds one research note that explains the Claude prompting guide lines as data. It proposes a diet for three personas.

## Rationale
ADR-0133 asks for one persona per change and a baseline first. A proposal note meets the item without a persona edit.

## Changed Areas
- `docs/research/practice-claude-prompting-guide-2026-11.md` — new note with the guide lines, the decision and the persona proposals.
- `docs/research-index.md` — one new entry that links the note after the earlier entries.
- `.evolve/evals/practice-claude-prompting-guide-proposal.md` — new eval file that pins the deliverable of the task, so the auditor grades the note against the same acceptance.
- `go/acs/cycle2123/predicates_test.go` — new set of 14 ACS predicates that check the note, the index entry, the unchanged persona files and the sentence lint. The TDD phase wrote it; this build does not change it.

## Design Decisions
The note names builder, auditor and scout for a diet. It gives three personas a no-change verdict. The marker counts come from the persona files.

## Verification
The ACS predicates of the cycle check the note, the index, the unchanged persona files and the sentence lint.

## Compatibility
No persona file, code file or schema changes. The earlier note stays as it was.

## Limitations
The guide text past the first 39,883 characters came from a model summary. The note does not rely on that part.
