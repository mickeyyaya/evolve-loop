# Build Explanation — Cycle 2276

## Build Binding
- Cycle: 2276
- Base SHA: 05f6fe2f97668c477a87e5bc9a78b191c4d4689f

## Summary
The build records the review of the Claude release notes for Fable 5.2 and Mythos 5.2. It adds one notes bullet, one research note and one index entry. No persona changes.

## Rationale
The inbox item asks for a data reading of the source and a proposal under ADR-0133. One release line gives a rule that an API caller can follow, the thinking prefix check. The notes file is the one home of such rules, so the smallest correct result is one bullet and a review record.

## Changed Areas
- `skills/model-notes-claude/COMPACT.md` — adds one bullet that names Fable 5.2, Mythos 5.2, thinking and the 400 error.
- `docs/research/practice-claude-model-release-notes-fable-5-2-2026-11.md` — new note that explains the lines as untrusted data and gives each core persona a no-change verdict.
- `docs/research-index/2026-11-10-04-practice-claude-model-release-notes-fable-5-2.md` — new index entry that links the note.

## Design Decisions
The notes file stays the one home of Claude practice. The note follows the layout of the earlier release notes note, so the review series stays uniform.

## Verification
The seven predicates of cycle 2276 pass, and the sentence lint reports 0 findings on the note, the index entry and the notes file.

## Compatibility
The five old bullets stay unchanged and the file stays below the ceiling of 12 lines.

## Limitations
The note does not measure a persona diet, because no D8 baseline exists. The page was read in part.
