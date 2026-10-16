# Build Explanation — Cycle 1912

## Build Binding
- Cycle: 1912
- Base SHA: e5761bb69b949ae6744b6f43841612a1ead9941d

## Summary
The Claude Code changelog research now reads release 2.1.302 as untrusted data, and each core persona weighs that release with a no-change verdict.

## Rationale
The practice watch filed the new changelog snapshot (hash e83d958f) as an inbox item. ADR-0133 asks for a review of each persona and no persona change before the M2 baseline. An extension of the existing research document keeps one document per source.

## Changed Areas
- `docs/research/practice-claude-code-changelog-2026-10.md` — adds the 2.1.302 section, the snapshot hash, the plan source of D7, D8, M2 and M3, and one 2.1.302 verdict per persona.
- `docs/research-index.md` — the existing entry now names the range 2.1.299 through 2.1.302.

## Design Decisions
The 2.1.302 section cites only identifiers that the snapshot text contains. The older release sections stay as they were. A new document per release was rejected, because the index then needs one row per release of one source.

## Verification
The cycle predicates `go test -tags acs -count=1 ./acs/cycle1912` pass, strict ste-lint reports zero findings on both files, and the practicewatch suite stays green.

## Compatibility
No persona, code or configuration changes. The index keeps its one link to the document.

## Limitations
The verdicts are reasoned, not measured. The M2 baseline must exist before a persona diet can change any prompt.
