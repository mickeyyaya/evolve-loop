---
score_cap:
  - criterion: "Account re-derives the boundary from Scope.Protected whatever the producer's label: a protected path labelled discovered (or any non-boundary label) with keep or carve is refused, while boundary+refuse, a declared in-scope protected path and an unprotected corroborated keep still account"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestAccount_ProtectedPathIsBoundaryWhateverTheLabel$' ./internal/scopedelta/"
  - criterion: "gate configuration (go/.apicover-enforce, go/go.mod, go/go.sum) is never closure, so an edit that removes another package's line is unaccounted without a decision and refused as a self-declared closure, while a corroborated tightening accounts"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^(TestAccount_GateConfigurationEditNeedsCorroboration|TestDefaultClosureRules_GateConfigurationIsNeverClosure)$' ./internal/scopedelta/"
  - criterion: "GamingSignals computes its floor and majorities over counted (non in-scope, non closure) entries only, so padding neither dilutes a majority nor lifts a lone entry over the floor"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestGamingSignals_MajoritiesIgnoreSkippedEntries|TestGamingSignals_PaddingCannotLiftALoneEntryOverTheFloor)$' ./internal/scopedelta/"
  - criterion: "the why that gate configuration is signal lives in TestSurfaceOf_GateConfigurationIsSignal and gaming.go carries no comment"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1796_004_GateConfigurationWhyIsATestNotAComment$' ./acs/cycle1796"
  - criterion: "the whole scopedelta package suite and vet stay green"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/scopedelta/... && go vet ./internal/scopedelta/..."
---

# Eval: scopedelta label and closure bypasses

> Pins the fix for inbox item scopedelta-label-and-closure-bypasses (cycle 1796, security class, from PR #749 comment round 12). Two holes let a producer ship work the adjudicator should have stopped. First, `Account` re-checked a declared closure or in-scope class but never the boundary class, and `Entry.Validate` refuses a keep only when the record already says boundary, so a protected path labelled `discovered` with keep, a declared tightens and any corroboration accounted clean. Second, `goBuildMetadataRule` made `go/.apicover-enforce`, `go/go.mod` and `go/go.sum` closure in any Go-touching cycle, and `Admissible` exempts closure from evidence, so an enrollment edit that deletes other packages' lines shipped uncorroborated. Third, `GamingSignals` divided by `len(entries)`, including the in-scope and closure entries it skips, so padding diluted a majority.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| label-bypass | protected path is boundary whatever the label | 9/10 | `go test -run '^TestAccount_ProtectedPathIsBoundaryWhateverTheLabel$' ./internal/scopedelta/` |
| closure-bypass | gate configuration needs a corroborated decision | 9/10 | `go test -run '^(TestAccount_GateConfigurationEditNeedsCorroboration\|TestDefaultClosureRules_GateConfigurationIsNeverClosure)$' ./internal/scopedelta/` |
| padding-dilution | majorities and floor over counted entries | 8/10 | `go test -run '^(TestGamingSignals_MajoritiesIgnoreSkippedEntries\|TestGamingSignals_PaddingCannotLiftALoneEntryOverTheFloor)$' ./internal/scopedelta/` |
| why-as-test | comment above signalFileHints replaced by a test | 5/10 | `go test -tags acs -run '^TestC1796_004_GateConfigurationWhyIsATestNotAComment$' ./acs/cycle1796` |
| suite-green | package tests and vet | 7/10 | `go test -count=1 ./internal/scopedelta/... && go vet ./internal/scopedelta/...` |

## Graders

- `[code]` cd go && go test -count=1 -run 'TestAccount|TestSurfaceOf_GateConfigurationIsSignal|TestGamingSignals|TestDefaultClosureRules' ./internal/scopedelta/...  (expect ok)
- `[code]` cd go && go test -tags acs -count=1 ./acs/cycle1796  (expect ok)
- `[code]` cd go && go test -count=1 ./internal/scopedelta/... && go vet ./internal/scopedelta/...
- `[model]` Protected path labelled discovered+keep is refused; .apicover-enforce line removal needs corroboration; skipped entries excluded from majority denominators and the floor.
