# Build Explanation — Cycle 2279

## Build Binding
- Cycle: 2279
- Base SHA: 0dfcb5c9b773355a13bf521750612239399eb1e3

## Summary
A research note reviews releases 2.1.321 and 2.1.322 of the Claude Code changelog under ADR-0133. An index entry links the note. No persona and no prompt changes.

## Rationale
The changelog lines are product fixes and one default model fact. The model notes already hold two bullets on Fable 5.2, so the notes need no new line. ADR-0133 requires a baseline before any persona diet, so each core persona gets a no-change verdict.

## Changed Areas
- `docs/research/claude-code-changelog-2-1-322.md` — reads the changelog at the link as untrusted data, explains the loop-relevant lines and gives a no-change verdict for each core persona.
- `docs/research-index/2026-11-10-06-practice-claude-code-changelog-2-1-322.md` — links the research note from the index.

## Design Decisions
A note without a notes edit is the smallest change. The proposal allowed at most one added line in the model notes, and the existing bullets already cover Fable 5.2. A persona change was rejected because no line shows a measured gain.

## Verification
The cycle predicates pass 9 of 9, and the ACS suite passes with red=0 on the second run. The sentence lint reports 0 findings on the note.

## Compatibility
The model notes file stays unchanged. No file under agents/, skills/ or go/ changes outside the cycle predicates.

## Limitations
The generated region of the research index is not regenerated in this cycle. The first ACS suite run showed three timing reds in other lanes, and they passed when run alone and in the second suite run.
