# Build Explanation — Cycle 2195

## Build Binding
- Cycle: 2195
- Base SHA: 7a57fb3e3cf44e370d226056b4f17d5486fd45fc

## Summary
Two documents said that the model-currency C5 work was not built. The code was built in cycle 2145. This change makes the documents match the code.

## Rationale
The four acceptance criteria already hold in code, and four predicates prove them. A new code change duplicates shipped work. The smallest correct change fixes the stale text.

## Changed Areas
- `docs/plans/model-currency-2026-10.md` — the stall note and the C5 list now state what is built, with code references.
- `docs/operations/runtime-reference.md` — the "Boundary CLI update" row now states the 20-minute boundary budget.
- `docs/explain/builds/cycle-2195-01m6qw26bmzz44mf6y0ewmsx3y.md` — this explanation.

## Design Decisions
The budget stays a compiled default plus the `Seams.Budget` parameter. A policy dial is not an acceptance criterion, so it is a new item if the operator wants it.

## Verification
The ACS suite for cycle 2195 runs the named bridge, tokenusage and cliupdate tests. The STE lint checks the new text.

## Compatibility
No code, schema or setting changed.

## Limitations
This change edits documents only. Ship consumes the inbox item: it moves the item to the processed folder after the cycle ships.
