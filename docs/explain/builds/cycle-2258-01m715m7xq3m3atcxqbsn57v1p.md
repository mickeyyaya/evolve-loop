# Build Explanation — Cycle 2258

## Build Binding
- Cycle: 2258
- Base SHA: c742075aa60af75d2f08b9172613ea49ae4eeb2d

## Summary
A new research note reads the agy CLI changelog page lines of one practice-watch item as untrusted data. It states that no prompt or persona changes under ADR-0133.

## Rationale
The item shows navigation labels, the releases v1.4.12 and v1.4.11, and a cut line. None of them touches a phase rule. ADR-0133 requires the M2 baseline before a diet, so a persona change is not allowed now. A new note keeps one document for the CLI tab and leaves the earlier notes unchanged.

## Changed Areas
- `docs/research/agy-cli-changelog-2026-11.md` — new note. It holds the source, the facts F1 to F8, the verdict for each core persona, and Decision D1 with two rejected options.
- `docs/research-index/2026-10-12-03-the-loop-watches-cli-practice-and-trims-its.md` — the entry file of the ADR-0133 section. It holds the source row of the new link, so the generated index keeps it.
- `docs/research-index.md` — adds one link to the note in the ADR-0133 section, so a reader can find it.
- `.evolve/evals/practice-agy-cli-changelog-page-a64d2284-202611040037.md` — the eval file that pins the note. It is a test input and is not edited in this build.
- `go/acs/cycle2258/predicates_test.go` — the predicates of the cycle. They are not edited in this build.

## Design Decisions
One note for the CLI tab keeps the App tab note and the CLI tab note apart. The verdict reuses the persona structure of the earlier notes, so the notes stay comparable.

## Verification
The sentence lint reports 0 findings for the note and the index. The predicates check the note text, the index link, the unchanged persona files and the unchanged earlier notes.

## Compatibility
No persona, skill or code file changes. The earlier notes stay as they were.

## Limitations
The page is long, and the read covered the CLI tab and the lines about the three features. The later part of the page was not read.
