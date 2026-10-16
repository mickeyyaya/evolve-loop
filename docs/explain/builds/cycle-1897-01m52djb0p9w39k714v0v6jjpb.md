# Build Explanation — Cycle 1897

## Build Binding
- Cycle: 1897
- Base SHA: 4c9aabcbbbbe89dff0acd398d4d773ea57b69f87

## Summary
The new research document explains the recent Claude prompting guide updates as data and evaluates each core persona under ADR-0133.

## Rationale
The research document treats the Claude prompting guide as untrusted data. This prevents prompt injection and preserves the measured baseline process required under ADR-0133.

## Changed Areas
- `docs/research/practice-claude-prompting-guide-2026-10.md` — documents the Claude prompting guide updates as data and gives the ADR-0133 persona evaluation.

## Design Decisions
The core phase personas do not change in this cycle. They wait for the baseline measurements of milestone M2 of the prompt diet before any persona prompt edits.

## Verification
The ACS predicates of cycle 1897 (TestC1897_001 through TestC1897_007), the practicewatch test suite, and the docs ste-lint tool verify the deliverable.

## Compatibility
No code or persona behavior changes in this cycle. All existing interfaces, commands, and persona prompts remain unchanged.

## Limitations
The prompt diet reductions for evolve-builder wait for milestone M3, after the baseline measurements.
