# Build Explanation — Cycle 1808

## Build Binding
- Cycle: 1808
- Base SHA: c03f7d4bb0294a4d7dd6c02a64b2dbd7d61351b9

## Summary
The inboxbatch defects of the cycle 1804 continuation are closed: UTF-8-safe truncation, first-wins duplicate id resolution everywhere, every path token of a files[] entry read. Coded plan-time inbox load warnings need the protected loopwave dispatch path and are left to console work.

## Rationale
Each fix reuses an existing seam: a rune-boundary back-off, a guarded index insert, one pathTokens helper shared by the file-area and operator-state rules, and a code prefix on the warning the gate already prints.

## Changed Areas
- `go/internal/inboxbatch/item.go` — field truncation backs up to a UTF-8 rune start so a cut never splits a rune.
- `go/internal/inboxbatch/rules.go` — indexByID keeps the first holder of an id; the new pathTokens splits an entry into path words minus wrapping punctuation and :line suffix, and the file-area rule counts every token.
- `go/internal/inboxbatch/archetype.go` — IsOperatorState requires every path token of every entry to sit under .evolve/, and an entry with no path token is not operator state.
- `go/internal/inboxbatch/unified.go` — Validate resolves a duplicated id to the first item through the shared indexByID, like connects and routing do.
- `go/acs/cycle1808/predicates_test.go` — the TDD-authored predicates TestC1808_001-015 pinning the above.
- `.evolve/evals/inboxbatch-utf8-and-resolution.md` — the eval graders for this task.
- `docs/private/research/archived-2026-10-06/superseded-predicate-packages/cycle1804/predicates_test.go` — the superseded cycle 1804 predicate package, archived by move.
- `docs/private/research/archived-2026-10-06/unshipped-build-explanations/cycle-1804-01m47mf94ecsjtysnp7afp8hbm.md` — the unshipped cycle 1804 explanation, archived by move.

## Design Decisions
Operator state stays conservative: a mixed entry or one with no path token answers false, since a false positive would skip review of a source change.

## Verification
ACS TestC1808_001-012, the inboxbatch, sizeratchet and loopwave suites pass with -count=1; TestC1808_013-015 stay red until the console edits loopwave.

## Compatibility
No exported signature changes.

## Limitations
A path containing spaces is split at the space.
