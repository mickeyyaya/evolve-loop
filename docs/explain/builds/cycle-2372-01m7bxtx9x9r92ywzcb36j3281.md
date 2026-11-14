# Build Explanation — Cycle 2372

## Build Binding
- Cycle: 2372
- Base SHA: cc99471bd4ad1ba17c3c4727e78eab4dbc460f1e

## Summary
The research note on the Claude prompting guide now answers the third practice report, item 526431cf. The stored excerpt has no added line, so the note guesses no change and proposes no persona edit this cycle.

## Rationale
The guide was read again at its link and treated as untrusted data. Each quoted line repeats a line that earlier sections of the note already explain. A new section keeps the record complete without a second note.

## Changed Areas
- `docs/research/practice-claude-prompting-guide-2026-11.md` — adds section 8 with six explained guide lines and the verdict table for six core personas under ADR-0133. It also gives the goal, done criteria, limits and before measure of three later diets.
- `docs/research-index/2026-11-14-03-practice-claude-prompting-guide-third-report.md` — adds the research-index entry that links the note for this report.
- `docs/research-index.md` — adds the section "2026-11-14 — Claude prompting guide, third report" with one table row at line 744 onward, inside the GENERATED research-index block. The row links the note. The index generator writes this content from the entry file above.

## Design Decisions
Section 8 extends the existing note, so the F rows and G rows stay unchanged. The verdicts reuse the order of section 4: builder, then auditor, then scout. The marker counts come from the persona files today.

## Verification
The nine ACS predicates of cycle 2372 pass. The STE lint reports 0 findings on both changed documents.

## Compatibility
No persona file or kernel code changes. The only generated file that changes is `docs/research-index.md`, and only inside its GENERATED block. The model notes file keeps its header and its lines.

## Limitations
The watcher sends an empty body, so the note cannot name the changed guide line. A separate item can track the watcher.
