# Build Explanation — Cycle 2489

## Build Binding
- Cycle: 2489
- Base SHA: 5db58d6b91d602b28eea4674bfd2dcbb3d18e095

## Summary
The new package `internal/testaudit` finds weak Go tests with 13 `go/ast` rules and ratchets the findings against a baseline file. The new package `internal/fakecontract` holds the marker type for a fake-tool contract suite.

## Rationale
The audit reads the syntax tree, so text inside a raw string is never a finding. A finding holds a package, a test and a rule, with no line number, so a moved test keeps its baseline row. A baseline row without a finding fails the ratchet, so fixed tests leave the baseline.

## Changed Areas
- `go/internal/testaudit/config.go` — config types, the embedded `fakes.json` and `guards.json`, and the bounds check.
- `go/internal/testaudit/analyze.go` — directory walk, package loading and the fake-row suite check.
- `go/internal/testaudit/rules.go` — the 13 rules and the shared constant resolver.
- `go/internal/testaudit/ratchet.go` — `WriteShard` and `CheckShard`.
- `go/internal/testaudit/fakes.json` — rows for git, tmux, gh and claude.
- `go/internal/testaudit/guards.json` — the empty guard list with its bound.
- `go/internal/testaudit/apicover_named_test.go` — names each export and checks the bounds and the empty shard.
- `go/internal/fakecontract/fakecontract.go` — the `Suite` marker type.
- `go/internal/fakecontract/apicover_named_test.go` — names `Suite`.
- `go/.apicover-enforce` — enrolls both packages in the apicover gate.
- `docs/architecture/packages/internal-testaudit.md` — package note.
- `docs/architecture/packages/internal-fakecontract.md` — package note.

## Design Decisions
Rules are data in one table. A guard row matches by package and test, never by code shape. A package-level suite check lives in the same walk as the rules, so one call gives all findings.

## Verification
The unit tests of both packages pass. All eleven ACS predicates pass, including the red and green fixtures of the `unverified-fake` rule in predicate 002 after the TDD fixture repair.

## Compatibility
Nothing outside the two new packages and the enforce list changes.

## Limitations
No production code calls the package yet. The `evolve test audit` command that calls it is a later component.
