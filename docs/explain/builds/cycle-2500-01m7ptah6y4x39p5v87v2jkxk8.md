# Build Explanation — Cycle 2500

## Build Binding
- Cycle: 2500
- Base SHA: b27e13c68c27e597197d10e5254a751284b51eff

## Summary
The apicover tool no longer counts an export as named when a test only binds it to the blank identifier. A call or a value assertion still names it.

## Rationale
A line such as `var _ = Exported` proves nothing about behavior. It made the unnamed-export gate pass without any assertion. Skipping bare references on the right side of a blank binding closes this gap with a small change in one function.

## Changed Areas
- `go/internal/apicover/refs.go` — `NamesReferencedInTests` skips bare identifiers and selectors on the right side of `var _ = X` and `_ = X`. Calls on that side still count. The declared type of `var _ T = v` still names `T`. Four unexported helpers support the check.
- `go/internal/apicover/refs_test.go` — table cases for blank bindings, including the typed assertion `var _ T = v`.
- `go/internal/inboxfile/apicover_named_test.go` — calls `IsLifecycleOwned` and asserts the two sentinel errors, which only a blank assertion named before.
- `go/internal/treefence/apicover_named_test.go` — asserts that the diagnostic message carries `RestoredMarker`.
- `go/internal/inboxmover/lifecycle/failure_record_test.go` — compares the route field against `RouteConsoleValue`.

## Design Decisions
The rule is syntactic and stays inside the existing AST walk. A call expression on the right side is still inspected, so `var _ = Exported()` names the symbol. No new export and no new package were added.

## Verification
The nine predicates TestC2500_001 to TestC2500_009 pass. The last three pin the typed blank assertion. The cycle ACS suite reports red=0. The `go test ./internal/apicover` run passes. The enforced package list reports no unnamed export.

## Compatibility
The public API of the package is unchanged. A package that relied on `var _ = X` to name an export now reports it as unnamed.

## Limitations
The rule does not detect other no-op uses, such as passing the symbol to an empty function.
