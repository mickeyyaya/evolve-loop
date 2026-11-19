---
score_cap:
  - criterion: "the registry test of internal/deliverable fails, and does not skip, on a corrupt or missing registry"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2539_001_' ./acs/cycle2539"
  - criterion: "the routing-order test of cmd/evolve fails, and does not skip, when the registry or the user specs cannot be loaded"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2539_003_' ./acs/cycle2539"
  - criterion: "the judgment-catalog test of internal/core fails, and does not skip, when the phase catalog is absent or empty"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2539_004_' ./acs/cycle2539"
  - criterion: "no skip-hides-error finding and no baseline row remain for the three tests"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2539_005_' ./acs/cycle2539"
---

# Eval: skips that hide a failed load of repository data fail the test (T3)

> Pins the test part of T3 of docs/plans/test-quality-audit-2026-11.md (D5). A test that loads the registry or the phase catalog of this repository must fail when the load fails. A skip there keeps a corrupt registry green. The predicates build the real test binary of each package and run it in a fixture tree with a corrupt, missing or empty input. The binary must exit non-zero with a FAIL line. Source incident: the findings of rule skip-hides-error in the shard baselines of cmd/evolve, internal/deliverable and internal/core (cycle 2539).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| corrupt-registry-deliverable | fails on a corrupt or missing registry | 7/10 | `go test -tags acs -run TestC2539_001_ ./acs/cycle2539` |
| routing-order | fails when registry or specs cannot load | 7/10 | `go test -tags acs -run TestC2539_003_ ./acs/cycle2539` |
| judgment-catalog | fails when catalog is absent or empty | 7/10 | `go test -tags acs -run TestC2539_004_ ./acs/cycle2539` |
| baseline-clean | no finding, no baseline row for the three tests | 6/10 | `go test -tags acs -run TestC2539_005_ ./acs/cycle2539` |
