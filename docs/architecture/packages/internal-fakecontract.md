# internal/fakecontract

## Purpose

`internal/fakecontract` names the test suite that checks a fake command-line tool against the real tool. The type `Suite` has the field `Tool`.

## Design

- A package that holds a `fakecontract.Suite{Tool: "git"}` literal states that a suite for `git` exists there.
- `internal/testaudit` reads the literal from the syntax tree. A row of `fakes.json` with a suite directory fails the `unverified-fake` rule when the literal is missing.

## Invariants

- The package has no behavior. The literal is the marker.

## Callers

No production code calls this package. Only the analyzer of `internal/testaudit` looks for its literal.
