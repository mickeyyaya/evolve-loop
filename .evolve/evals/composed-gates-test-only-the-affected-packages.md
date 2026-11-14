---
score_cap:
  - criterion: "A one-package change scopes the composed test gate to that package and its importers"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestComposedGateScope_TestGateRunsOnlyChangedAndImporters$'"
  - criterion: "The composed apicover gate checks only the enforced packages inside the closure"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestComposedGateScope_ApicoverGateRunsOnlyEnforcedEntriesInTheClosure$'"
  - criterion: "GateScope returns the changed package and its importers only"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./internal/changedpkgs -run '^TestGateScope_OnePackageChangeRunsThatPackageAndItsImporters$'"
  - criterion: "A go.mod change, an unreadable graph or an underivable change falls back to the full set"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 ./internal/changedpkgs -run '^(TestGateScope_ModuleRootChangeFallsBackToTheFullSet|TestGateScope_UnderivableChangeFallsBackToTheFullSet|TestGateScope_UnreadableImportGraphFallsBackToTheFullSet)$'"
  - criterion: "A change with no Go file skips the scoped gates instead of testing the current directory"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestComposedGateScope_ChangeOutsideGoSkipsTheScopedGatesAndSaysSo$'"
  - criterion: "The design doc compares three scopes with measured CPU and escape risk"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2375 -run '^TestC2375_001_'"
---

# Eval: composed gates test only the affected packages

> Pins the scoped composed-tree test and apicover gates of cycle 2375. The gates ran `make test` and `make apicover-enforce` over the whole module on every composed tree. Each run costs the CPU of the full suite for a change that touches one package. The scope is the importer closure of the changed packages, with a full-set fallback whenever git or `go list` cannot derive it.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| scoped-test-gate | Only the changed package and its importers reach `FAST_PKGS` | 3/10 | `TestComposedGateScope_TestGateRunsOnlyChangedAndImporters` |
| scoped-apicover-gate | Only enforced entries inside the closure reach `APICOVER_PKGS` | 4/10 | `TestComposedGateScope_ApicoverGateRunsOnlyEnforcedEntriesInTheClosure` |
| closure-derivation | `GateScope` returns the changed package and its importers | 4/10 | `TestGateScope_OnePackageChangeRunsThatPackageAndItsImporters` |
| fail-toward-full | An underivable change never runs less than the full set | 3/10 | `TestGateScope_UnderivableChangeFallsBackToTheFullSet` |
| no-go-change | An empty scope skips the run, because `go test` with no argument tests the current directory | 4/10 | `TestComposedGateScope_ChangeOutsideGoSkipsTheScopedGatesAndSaysSo` |
| design-doc | Three scopes, measured CPU, escape risk | 5/10 | `TestC2375_001_TheDesignDocComparesThreeScopesWithMeasuredCPUAndEscapeRisk` |
