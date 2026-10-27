---
score_cap:
  - criterion: "A regression package whose module-local graph imports no escape hatch lands in WouldSkip for a change outside its dependencies"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2049_001' ./acs/cycle2049 && go test -count=1 -run 'TestCompute_SkipsAPredicateThatReachesOsOnlyThroughTesting' ./internal/regressiontia"
  - criterion: "A package that reaches os through a module package, directly or through a chain, stays selected"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2049_002' ./acs/cycle2049"
  - criterion: "The shadow soak rollup records would_skip greater than zero for a change that touches no dependency of some regression package"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2049_003' ./acs/cycle2049"
  - criterion: "A change to a dependency of a regression package still selects it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2049_004' ./acs/cycle2049"
---

# Eval: Regression TIA escape-hatch check reads module-local dependencies

> Pins the fix for inbox item regression-tia-escape-hatch-is-transitive (cycle 2049). `listCorpusDeps` lists the transitive `.Deps`, so every test package reaches `os` through `testing` and the selection never skips a package. The fix walks only module packages and checks each one for a direct escape-hatch import.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| skippable-without-hatch | A package with no hatch in its module-local graph is skippable | 4/10 | `go test -tags acs -run TestC2049_001 ./acs/cycle2049` |
| hatch-through-module-package | Reaching os through a module package stays a hatch | 6/10 | `go test -tags acs -run TestC2049_002 ./acs/cycle2049` |
| soak-records-skip | The rollup records would_skip above zero | 5/10 | `go test -tags acs -run TestC2049_003 ./acs/cycle2049` |
| dependency-change-selects | A changed dependency still selects the package | 6/10 | `go test -tags acs -run TestC2049_004 ./acs/cycle2049` |
