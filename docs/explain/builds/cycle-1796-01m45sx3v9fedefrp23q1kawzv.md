# Build Explanation — Cycle 1796

## Build Binding
- Cycle: 1796
- Base SHA: 0f072732bb7a4493220ded7239c3a20758687d94

## Summary
The scope-delta adjudicator closes three ways a producer could ship work it should have been stopped on. `Account` now works out the boundary class from `Scope.Protected` instead of trusting the entry's label. Gate configuration (`go/.apicover-enforce`, `go/go.mod`, `go/go.sum`) is no longer closure, so editing it needs a corroborated decision. `GamingSignals` now computes its floor and its majorities over the entries it actually counts.

## Rationale
- **Label bypass.** `Entry.Validate` refuses a non-refuse disposition only when the record already says `boundary`, and `Account` re-checked the `closure` and `in-scope` labels but never the `boundary` one. So a protected path labelled `discovered`, with a declared `tightens` and any corroboration, accounted clean. Working out the class from the scope is the same move the package already makes for closure: the class is computed, never claimed.
- **Gate configuration as closure.** In any Go-touching cycle, `goBuildMetadataRule` made the three gate-configuration files closure. Closure skips the evidence layer, so an enrollment edit that drops another package's `.apicover-enforce` line shipped with no corroboration at all.
- **Padding dilution.** `GamingSignals` skipped `in-scope` and `closure` entries, but it still divided by `len(entries)`. That meant adding mechanically established entries diluted every majority, and it also let a single counted entry clear the three-entry floor.

## Changed Areas
- `go/internal/scopedelta/scopedelta.go`:
  - `Account` refuses any entry for a protected path, outside the declared scope, that is not `boundary`/`refuse`, and the error names the path.
  - `goBuildMetadataRule` and its `touchesGo` helper are deleted, which drops the rule from `DefaultClosureRules`.
  - `coveredByRule` never covers a protected path the scope does not declare. Without that, a path that a closure rule matched could be left out of the record and slip past the boundary that the new `Account` check refuses for a labelled entry.
- `go/internal/scopedelta/gaming.go`:
  - `GamingSignals` counts the entries it does not skip, applies the floor to that count, and uses that count as every majority's denominator and in every message.
  - The one-line comment above `signalFileHints` is removed. Its reason now lives in `TestSurfaceOf_GateConfigurationIsSignal`.
- `go/internal/scopedelta/account_test.go`:
  - Holds the TDD-authored `TestAccount_ProtectedPathIsBoundaryWhateverTheLabel`, `TestAccount_GateConfigurationEditNeedsCorroboration` and `TestDefaultClosureRules_GateConfigurationIsNeverClosure`. The last one replaces the old test that pinned gate configuration as closure.
  - Adds the builder's `TestAccount_ProtectedPathAClosureRuleWouldCoverStillNeedsARefusal`, which pins the `coveredByRule` guard. It was checked red without the guard and green with it.
- `go/internal/scopedelta/gaming_test.go` — TDD-authored `TestSurfaceOf_GateConfigurationIsSignal`, `TestGamingSignals_MajoritiesIgnoreSkippedEntries` and `TestGamingSignals_PaddingCannotLiftALoneEntryOverTheFloor`.
- `go/acs/cycle1796/predicates_test.go` — the five TDD-authored acceptance predicates for this item.
- `.evolve/evals/scopedelta-label-and-closure-bypasses.md` — the eval for this item (TDD-authored, tracked with the build).
- `docs/architecture/packages/internal-scopedelta.md` — the closure-rule list drops `go-build-metadata` and says why gate configuration is never closure. The page also documents:
  - `Account`'s boundary re-derivation, including in the "record cannot overturn policy" invariant;
  - the protected-path exclusion in closure;
  - the counted-only floor and denominators of `GamingSignals`;
  - the test that now pins the gate-configuration signal reason.

## Design Decisions
- **Where the boundary check sits.** It runs in `Account`, before the closure and in-scope checks. It does not run in `Validate`, because `Validate` has no `Scope` and judges only how the record is stated. Putting it first also catches a protected path labelled `closure` that a rule would otherwise cover.
- **Scope precedence matches `Classify`.** A protected path that the scope itself declares stays in-scope, the same way `Classify` ranks in-scope ahead of protected. The operator's declaration licenses it.
- **Delete the rule, don't narrow it.** The rule was deleted rather than limited to additions, because a path rule cannot tell an added enrollment line from a removed one. A genuine enrollment still ships. It takes a `discovered` keep marked `tightens` with a counterfactual, which the existing `Admissible` signal-surface logic already supports, and a `loosens` keep is carved.
- **Rejected alternative.** One option was to keep gate configuration as closure and add a content check to `Admissible`. `Admissible` sees only the `Entry` and never the diff, so this would have meant a new input surface for a judgement that adjudication already makes.

## Verification
- `cd go && go test -count=1 ./internal/scopedelta/... && go vet ./internal/scopedelta/...` passes.
- `cd go && go test -tags acs -count=1 ./acs/cycle1796` passes all five predicates.
- `gofmt -l` is clean for the touched packages.
- The new `coveredByRule` regression test fails with the guard disabled and passes with it in place.

## Compatibility
- `scopedelta` has no production importer outside its own package and the ACS predicates, so no caller changes behavior in this cycle.
- `DefaultClosureRules` returns two rules instead of three.
- For a future caller, a Go cycle that enrolls a new package must now record a corroborated decision for its `.apicover-enforce` line.

## Limitations
- No path rule can tell whether an `.apicover-enforce` edit adds a line or removes one. That stays a judgement made in adjudication, backed by the corroboration command.
- `GamingSignals` still reports shape only. It does not block.
