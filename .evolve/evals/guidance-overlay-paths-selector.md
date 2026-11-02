---
score_cap:
  - criterion: "An overlay rule with Paths loads only for a scope that matches, and an empty Paths matches every scope"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2091_00[1-4]' ./acs/cycle2091"
  - criterion: "A malformed Paths glob fails at policy load and the parsed rule selects by scope"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2091_005 ./acs/cycle2091"
  - criterion: "The compiled default code rule keeps its three skills until the dispatch wiring lands"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC2091_006 ./acs/cycle2091"
  - criterion: "The changed path functions have 100 percent coverage and the package note and spec describe the selector"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2091_00[78]' ./acs/cycle2091"
---

# Eval: Guidance G4 overlay Paths selector

> Pins the policy-side half of guidance-overlay-paths-selector (ADR-0145, plan D5):
> `OverlayRule.Paths` and `OverlayDispatch.ScopePaths` select a rule by the paths of the lane scope.
> The runner wiring in `dispatch.go` and the split of the default rule are console work and stay out of this eval.
> Source incident: research F2.8, where docs-write reached every code-writing dispatch.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| selector-semantics | Paths loads only for a matching scope; empty Paths matches all | 6/10 | `go test -tags acs -run 'TestC2091_00[1-4]' ./acs/cycle2091` |
| load-validation | A malformed glob fails at load | 7/10 | `go test -tags acs -run TestC2091_005 ./acs/cycle2091` |
| default-preserved | The default code rule is unchanged | 6/10 | `go test -tags acs -run TestC2091_006 ./acs/cycle2091` |
| coverage-and-docs | 100 percent function coverage; note and spec updated | 5/10 | `go test -tags acs -run 'TestC2091_00[78]' ./acs/cycle2091` |
