# Build Explanation — Cycle 2256

## Build Binding
- Cycle: 2256
- Base SHA: 4c94d55f402f042772c04500f85744500c970bd3

## Summary
The research note for the Claude prompting guide now records the second practice-watch report, item 8d950f1e. It states that no persona file changes in this cycle.

## Rationale
The item snapshot body is empty, so it names no changed line. The guide was read again at its link. Its lines on aggressive wording repeat lines that the note already explains. The existing proposal stands. A new section keeps one note per source.

## Changed Areas
- `docs/research/practice-claude-prompting-guide-2026-11.md` — adds section 7. It holds the item id, the compared snapshot and the empty-body fact. It also holds the guide lines read as untrusted data and the decision to change no persona now.
- `docs/explain/builds/cycle-2256-01m714ssrsgtb75418c3wq97pa.md` — this explanation of the build.
- `solutions/practice-claude-prompting-guide-8d950f1e-202611060247/proposal.md` — the proposal, with the persona verdicts under ADR-0133.
- `solutions/practice-claude-prompting-guide-8d950f1e-202611060247/recommendation.md` — compares three options and picks Option 1.
- `solutions/practice-claude-prompting-guide-8d950f1e-202611060247/assumptions-and-evidence.md` — the assumptions A1 to A3 and the evidence E1 to E6.
- `solutions/practice-claude-prompting-guide-8d950f1e-202611060247/options/1-no-persona-change-record-verdict.md` — option 1, change no persona now.
- `solutions/practice-claude-prompting-guide-8d950f1e-202611060247/options/2-soften-builder-persona-now.md` — option 2, soften the builder now.
- `solutions/practice-claude-prompting-guide-8d950f1e-202611060247/options/3-marker-count-guard.md` — option 3, add a marker-count check.

## Design Decisions
The proposal of the note (builder, then auditor, then scout) stays as written. ADR-0133 rules D7 and D8 need a baseline and 10 cycles before and after, so a diet cannot start in this cycle.

## Verification
`go/evolve docs ste-lint --strict` on the note reports 0 findings. The text checks of the eval graders match the item id, the snapshot, the persona name and the data wording.

## Compatibility
Sections 1 to 6 of the note are unchanged. The earlier 2026-10 note is unchanged.

## Limitations
The page shows no update date, so the note cannot name the changed line. The empty snapshot body is a watcher defect that this cycle does not fix.
