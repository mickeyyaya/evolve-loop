# Build Explanation — Cycle 1915

## Build Binding
- Cycle: 1915
- Base SHA: fb7761a757eb659f91cf99fe9955220fb8bb1461

## Summary
The agy CLI changelog page research now reads release v2.24.0 as untrusted data, and each core persona weighs that release with a no-change verdict.

## Rationale
The practice watch filed the new changelog snapshot as an inbox item. ADR-0133 asks for a review of each persona and no persona change before the M2 baseline. An extension of the existing research document keeps one document per source.

## Changed Areas
- `docs/research/practice-agy-cli-changelog-page-2026-10.md` — adds the v2.24.0 section as data and records one v2.24.0 verdict per core persona.
- `docs/research-index.md` — the existing entry now names release v2.24.0.

## Design Decisions
The v2.24.0 section cites only identifiers that the snapshot text contains. Older release sections stay intact. A new document per release was rejected, because the index then needs one row per release of one source.

## Verification
The cycle predicates `go test -tags acs -count=1 ./acs/cycle1915` pass, strict ste-lint reports zero findings on both files, and the practicewatch suite stays green.

## Compatibility
No persona, code, or configuration changes. The index keeps its one link to the document.

## Limitations
The verdicts are reasoned, not measured. The M2 baseline must exist before a persona diet can change any prompt.
