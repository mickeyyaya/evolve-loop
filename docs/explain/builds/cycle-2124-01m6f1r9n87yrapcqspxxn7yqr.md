# Build Explanation — Cycle 2124

## Build Binding
- Cycle: 2124
- Base SHA: 3491a4878893c5cc4fac1dfaeef5baee90d5c58f

## Summary
The build adds a research note on the agy changelog page and links it in the research index. The note reads the releases v2.28.2, v2.28.1, v2.28.0 and v2.27.0 as untrusted data. It decides that no persona changes.

## Rationale
The practice-watch item asks for the page lines to be explained as data and for a proposal under ADR-0133. The live page shows that the four releases sit in the App panel, not in the CLI panel. This fact rules out any prompt rule for the built-in browser or the Plugin Marketplace. Rule D8 also requires the M2 baseline before any diet, so a persona change is not allowed yet.

## Changed Areas
- `docs/research/practice-agy-cli-changelog-page-2026-11.md` — new note with the source, the fact table, six persona verdicts and one decision with two rejected options.
- `docs/research-index/2026-10-12-03-the-loop-watches-cli-practice-and-trims-its.md` — adds the index entry of the note, and restores the entry of the prompting guide note that the generator had no source for.
- `docs/research-index.md` — generated again from the entry files; the only visible change is the new row of the note.

## Design Decisions
The index is generated, so the entry goes into the source entry file and the generator writes the table. The note does not touch the earlier notes, because a document is archived and never edited in place.

## Verification
The 14 predicates of cycle 2124 pass. The note and the index pass `evolve docs ste-lint --strict` with 0 findings.

## Compatibility
No persona file, no code and no earlier note changes. The index keeps every earlier row.

## Limitations
The note reads the page once, on 2026-11-03. The page can change after that date. The watch still files App releases as CLI updates; the note records this as a follow-up.
