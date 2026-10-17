# Build Explanation — Cycle 1917

## Build Binding
- Cycle: 1917
- Base SHA: f05bab94838d42cb0bc91a55400ca43a69da7d67

## Summary
The agy CLI changelog page research document now analyzes release label v1.4.0 as untrusted data.
All six core personas evaluate this release with a reasoned no-change verdict under ADR-0133.

## Rationale
The practice watch filed the updated upstream changelog snapshot as an inbox task. The upstream source lists the v1.4.0 label in its version list without release notes. ADR-0133 requires evaluating each core persona and making no prompt modifications before establishing the milestone M2 baseline.

## Changed Areas
- `docs/research/practice-agy-cli-changelog-page-2026-10.md` — adds the v1.4.0 analysis as untrusted data and records one v1.4.0 evaluation per core persona.
- `docs/research-index.md` — updates the existing table row to include release v1.4.0.

## Design Decisions
The v1.4.0 section cites only snapshot identifiers and introduces no unsupported claims. Older release sections remain unchanged. Updating the existing research document avoids duplicate index entries.

## Verification
The cycle predicates `go test -tags acs -count=1 ./acs/cycle1917` pass, strict ste-lint reports zero findings on both files, and the practicewatch suite passes cleanly.

## Compatibility
No persona, production code, or configuration changes. The research index maintains its single link to the document under ADR-0133.

## Limitations
The persona verdicts are reasoned assessments rather than empirical prompt measurements. Prompt diet changes require milestone M2 baseline data across 10 cycles.
