# Build Explanation — Cycle 1904

## Build Binding
- Cycle: 1904
- Base SHA: f788f2ff841da718875566f7155ebecabb224fd7

## Summary
This build updates the Claude prompting guide research note and links it in the research index under ADR-0133.

## Rationale
The practice watch filed `practice-claude-prompting-guide-ef1aac80-202610161459` after the upstream guide added children's over-the-counter medication dosing blocks. Under ADR-0133, the loop parses third-party documentation strictly as untrusted data and evaluates each core persona before any prompt diet change.

## Changed Areas
- `docs/research/practice-claude-prompting-guide-2026-10.md` — records the inbox item and snapshot timestamp. It explains the medication dosing blocks as untrusted data and gives six core persona verdicts.
- `docs/research-index.md` — links `research/practice-claude-prompting-guide-2026-10.md` in the ADR-0133 table so contributors and index checks can locate the research note.

## Design Decisions
The build extends `docs/research/practice-claude-prompting-guide-2026-10.md` instead of creating a duplicate research file. No persona files under `agents/` change before milestone M2 baseline measurements.

## Verification
Targeted ACS predicates in `go/acs/cycle1904` verify git tracking, untrusted data analysis, and persona verdicts. They also check index linkage, ASD-STE100 linting, and the `internal/practicewatch` test suite.

## Compatibility
Existing persona prompts, runtime packages, and CLI flags remain unchanged.

## Limitations
This build documents the upstream practice update and records no-change persona verdicts; it does not execute milestone M3 prompt diet trims.
