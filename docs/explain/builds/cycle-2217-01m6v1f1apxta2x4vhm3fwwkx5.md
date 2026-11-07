# Build Explanation — Cycle 2217

## Build Binding
- Cycle: 2217
- Base SHA: f2c168cac7992885a5a91681e7f7d8b967b10bb5

## Summary
Two documents said that the model-currency C5 work was not built. The code was built in cycle 2145. This change makes the documents match the code.

## Rationale
The four acceptance criteria already hold in code, and four predicates prove them. A new code change duplicates shipped work. The smallest correct change fixes the stale text.

## Changed Areas
- `docs/plans/model-currency-2026-10.md` — updates the stall note and the C5 item status to match the shipped code.
- `docs/operations/runtime-reference.md` — records the twenty-minute boundary update budget in the runtime reference.
- `docs/explain/builds/cycle-2217-01m6v1f1apxta2x4vhm3fwwkx5.md` — provides this explanation document for cycle 2217.
- `go/acs/cycle2217/predicates_test.go` — adds four Go ACS predicates that prove the four task criteria.
- `.evolve/evals/c5-served-model-verification.md` — defines the eval specification for served model verification.
- `.evolve/evals/n6-boundary-stall-budget.md` — defines the eval specification for the boundary stall budget.
- `docs/private/research/archived-2026-11-07/superseded-predicate-packages/cycle2195/predicates_test.go` — archives the unshipped cycle 2195 predicate tests per the archiving rule.
- `docs/private/research/archived-2026-11-07/unshipped-build-explanations/cycle-2195-01m6qw26bmzz44mf6y0ewmsx3y.md` — archives the unshipped cycle 2195 build explanation document.

## Design Decisions
The budget stays a compiled default plus the `Seams.Budget` parameter. A policy dial is not an acceptance criterion, so it is a new item if the operator wants it.

## Verification
The ACS suite for cycle 2217 runs the named bridge, tokenusage and cliupdate tests. The STE lint checks the new text.

## Compatibility
No production code, schema or setting changed. The diff adds Go ACS test predicates only.

## Limitations
This change adds test predicates and updates documentation without altering production binaries. Ship consumes the inbox item after the cycle ships.
