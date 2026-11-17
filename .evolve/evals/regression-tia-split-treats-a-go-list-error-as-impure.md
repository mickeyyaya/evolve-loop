---
score_cap:
  - criterion: "a corpus package that imports a package with a go list error is never would-skip and stays selected"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2459_001_' ./acs/cycle2459"
  - criterion: "a go list error two hops down the closure still makes the corpus package unskippable"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2459_002_' ./acs/cycle2459"
  - criterion: "an error-free corpus package with no escape hatch stays skippable beside errored ones"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2459_003_' ./acs/cycle2459"
  - criterion: "the regressiontia package suite stays green"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/regressiontia/"
---

# Eval: the TIA split treats a go list error as impure

> Pins the rule that the TIA split and the ship scan-skip purity read one
> escape-hatch and error rule. `PurePackages` treats a package with a go list
> error as impure, but the corpus walk (`listCorpusDeps` and the code that
> turns its closure into skippable dependencies) discarded the error, so a
> corpus package whose go list failed could be would-skip. Source incident:
> cycle 2459, architecture review of PR 1542. The predicates drive the exported
> `regressiontia.Compute`, which the audit phase calls (`tiaCompute` in
> `go/internal/phases/audit/audit.go`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| direct-error | a package importing an errored package is not would-skip | 5/10 | `go test -tags acs -run TestC2459_001_ ./acs/cycle2459` |
| transitive-error | an error two hops down still blocks the skip | 6/10 | `go test -tags acs -run TestC2459_002_ ./acs/cycle2459` |
| clean-control | an error-free package stays skippable | 6/10 | `go test -tags acs -run TestC2459_003_ ./acs/cycle2459` |
| package-suite | the package tests stay green | 7/10 | `go test ./internal/regressiontia/` |
