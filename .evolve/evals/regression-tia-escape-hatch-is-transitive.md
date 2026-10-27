---
score_cap:
  - criterion: "A regression package whose module-local graph imports no escape hatch lands in WouldSkip for a change outside its dependencies"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2053_001' ./acs/cycle2053 && go test -count=1 -run 'TestCompute_SkipsAPredicateThatReachesOsOnlyThroughTesting' ./internal/regressiontia"
  - criterion: "A package that reaches os or os/exec through a module package, directly or through a chain, stays selected"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2053_002|TestC2053_006' ./acs/cycle2053"
  - criterion: "The shadow soak rollup records would_skip greater than zero for a change that touches no dependency of some regression package"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2053_003' ./acs/cycle2053"
  - criterion: "A change to a dependency still selects the depending package, and a package with no module dependency still runs"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2053_004|TestC2053_005' ./acs/cycle2053"
  - criterion: "The regressiontia package is fully covered"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2053_007' ./acs/cycle2053"
---

# Eval: Regression TIA escape-hatch check reads module-local dependencies

> Pins the fix for inbox item regression-tia-escape-hatch-is-transitive (cycles 2049 and 2053). `listCorpusDeps` listed the transitive `.Deps`, so every test package reached `os` through `testing` and the selection never skipped a package. The fix walks only module packages and checks each one for a direct escape-hatch import. Cycle 2053 replaces the archived cycle-2049 predicates.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| skippable-without-hatch | A package with no hatch in its module-local graph is skippable | 4/10 | `go test -tags acs -run TestC2053_001 ./acs/cycle2053` |
| hatch-via-module-package | Reaching os through a module package stays a hatch, on a fixture and on the real tree | 6/10 | `go test -tags acs -run 'TestC2053_002\|TestC2053_006' ./acs/cycle2053` |
| soak-records-skip | The rollup records would_skip above zero | 5/10 | `go test -tags acs -run TestC2053_003 ./acs/cycle2053` |
| dependency-change-selects | A changed dependency still selects the package | 6/10 | `go test -tags acs -run 'TestC2053_004\|TestC2053_005' ./acs/cycle2053` |
| full-coverage | The package holds 100% statement coverage | 7/10 | `go test -tags acs -run TestC2053_007 ./acs/cycle2053` |
