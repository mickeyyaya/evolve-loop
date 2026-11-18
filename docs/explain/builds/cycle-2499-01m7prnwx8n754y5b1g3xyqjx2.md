# Build Explanation — Cycle 2499

## Build Binding
- Cycle: 2499
- Base SHA: 0f9537a5612b0c8c13acaede92bbff592b6e2a9d

## Summary
The TDD reviewer now refuses a new ACS predicate that only matches source text. A predicate that runs the code passes.

## Rationale
The lint follows the existing flaky and unsatisfiable lints. It reuses the same AST helpers and the same predicate lint gate, so no second mechanism exists. A predicate is refused when it calls acsassert.FileContains or acsassert.FileMatchesRegex and calls nothing that runs code.

## Changed Areas
- `go/internal/evalqualitycheck/sourcetext.go` — adds LintSourceTextPredicates, which finds Test functions that only grep source text.
- `go/internal/evalgate/sourcetext.go` — adds the source-text gate, which blocks at the enforce stage.
- `go/internal/evalgate/reviewer.go` — composes the new gate into NewReviewer.
- `go/internal/evalgate/gate_wiring_registry_test.go` — registers the wiring pin of the new gate.
- `go/internal/evalgate/sourcetext_test.go` — proves the gate is reached through NewReviewer.
- `go/internal/evalqualitycheck/apicover_named_test.go` — names the new exported symbols in a real assertion.

## Design Decisions
A predicate runs code in four cases.
- It calls exec.Command.
- It calls an acsassert Subprocess or Output helper.
- It calls a repo package other than acsassert.
- It calls a local helper that does one of these.

A doc comment with `acs-predicate: config-check` waives a config-presence check.

## Verification
The eight cycle 2499 predicates pass. The evalgate and evalqualitycheck packages pass. The ACS suite reports red=0.

## Compatibility
The lint reads only the predicate package of the current cycle. Older predicates are not linted.

## Limitations
A helper in another package that only greps text counts as running code. The lint does not follow calls across packages.
