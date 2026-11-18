---
score_cap:
  - criterion: "a package whose only test reference is var _ = Exported (package level or inside a test) reports the func and the const as not named"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2500_00(1|2|4)' ./acs/cycle2500"
  - criterion: "a test that calls an exported func or asserts an exported const value names it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2500_00(3|5|6)' ./acs/cycle2500"
  - criterion: "a typed blank assertion var _ T = v names the type T (and not the value v), including in the enforced packages scopedelta and mergegate"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2500_00(7|8|9)' ./acs/cycle2500"
---

# Eval: apicover counts an export as named only when a test calls it or asserts its value

> Pins the T2 rule of the test-quality audit. `NamesReferencedInTests` used to mark every identifier in a `_test.go` file as a use, so `var _ = Exported` named an export that no test ran. The predicates drive `apicover.Main` over temp modules. Source incident: the test-quality audit plan (PR #1562), item T2, cycle 2500. Audit round 1 found that dropping `ValueSpec.Type` made `var _ T = v` stop naming `T`; 007 to 009 pin that.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| blank-reference-negative | `var _ = Exported` and `_ = Exported` leave the export unnamed | 4/10 | `go test -tags acs -run 'TestC2500_00(1\|2\|4)' ./acs/cycle2500` |
| call-or-assert-positive | a call or a value assertion names the export | 6/10 | `go test -tags acs -run 'TestC2500_00(3\|5\|6)' ./acs/cycle2500` |
| typed-blank-assertion | `var _ T = v` names `T`, never `v` (audit round 1, H1) | 5/10 | `go test -tags acs -run 'TestC2500_00(7\|8\|9)' ./acs/cycle2500` |
