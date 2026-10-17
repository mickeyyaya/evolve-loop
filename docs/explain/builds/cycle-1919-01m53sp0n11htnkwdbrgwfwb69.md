# Build Explanation — Cycle 1919

## Build Binding
- Cycle: 1919
- Base SHA: bddb47ad7b50dbc7094f3869bc50c550c618fc68

## Summary
The agy changelog page research document now explains the three snapshot diff lines of 2026-10-17T02:04:09Z as data. Each core persona entry weighs the keyboard focus fix and keeps its no-change verdict.

## Rationale
The inbox item asks for the diff lines to be read as data and for a verdict on each persona under ADR-0133. The existing document already held the earlier release items, so an added section is the smallest change.

## Changed Areas
- `docs/research/practice-agy-cli-changelog-page-2026-10.md` — adds the snapshot diff section and one keyboard fix line to each persona entry, because the item asks for both.
- `docs/research-index.md` — the index line of the document names the snapshot diff, so a reader finds the new section.
- `go/acs/cycle1919/predicates_test.go` — the TDD phase wrote these predicates; the build changes no line of them.
- `.evolve/evals/practice-agy-cli-changelog-page-c71ffe53-202610170204.md` — the scout phase wrote this eval; the build changes no line of it.

## Design Decisions
The diff section cites only identifiers that the snapshot holds. The document states the two inferences about page growth as synthesis. No persona file changes, because ADR-0133 D8 needs the M2 baseline first.

## Verification
The 12 cycle predicates pass, and the ACS suite reports red=0. The ste-lint check reports zero findings on both documents.

## Compatibility
The document keeps every earlier release section, and the index keeps one link to it.

## Limitations
The snapshot holds only the first 64 KiB of the page, so the document cannot quote the full keyboard fix line from the snapshot.
