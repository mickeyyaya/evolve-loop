# Build Explanation — Cycle 2047

## Build Binding
- Cycle: 2047
- Base SHA: 0dc815909c3078ea8acb6c7f159e61ed09623053

## Summary
The retry ladder package now has a fix spec format, a reader for it and a scope guard for a fix diff.

## Rationale
Decisions D1 and D5 of the finder fix ownership plan need a fixed format and a deterministic guard. A later cycle wires rung R3 on them. A reader and guard in the existing package keep the ladder logic in one home.

## Changed Areas
- `go/internal/retryladder/fixspec.go` — adds the spec types, `ParseFixSpec` and `FixScopeViolations`.
- `go/internal/retryladder/apicover_named_test.go` — names each new export in a real assertion.
- `docs/architecture/packages/internal-retryladder.md` — records the format and the guard.

## Design Decisions
The guard compares hunk ranges, so a diff with no context lines gives an exact check. An insertion is inside an edit at the edge of its span. No new package is added, so the apicover enforce list is unchanged.

## Verification
The tests of the retryladder package and the cycle 2047 predicates cover the reader, the refusals and both scope rules.

## Compatibility
The change adds exports only. No existing behavior changes.

## Limitations
No production code calls the new exports yet. The later steps F1 and F2 of the plan wire them.
