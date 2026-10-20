---
score_cap:
  - criterion: "TestFlags* pass when a lane's EVOLVE_WORKTREE_ROOT reaches the test run"
    max_if_missing: 6
    evidence: "cd go && EVOLVE_WORKTREE_ROOT=/elsewhere go test -count=1 -run '^(TestFlagsGenerateThenCheck_RoundTrip|TestFlagsCheck_DriftExitsTwo)$' ./cmd/evolve/"
  - criterion: "TestPredicateEnv_AllBranches passes when a lane's EVOLVE_WORKTREE_ROOT reaches the test run"
    max_if_missing: 6
    evidence: "cd go && EVOLVE_WORKTREE_ROOT=/elsewhere go test -count=1 -run '^TestPredicateEnv_AllBranches$' ./internal/acssuite/"
  - criterion: "flags generate never writes outside the test's temp root"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2009_00[12]' ./acs/cycle2009"
---

# Eval: flags tests are hermetic against a lane's EVOLVE_WORKTREE_ROOT

> Pins that `seedFlagsProject` isolates `TestFlags*` from the lane's worktree root, so
> `runFlags generate` writes only into the test's temp project. Source incident:
> cycle 2009 lane item flags-and-predicate-tests-hermetic-env-lane (predicate half shipped in #1014).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| env-hermetic flags | TestFlags* pass with a foreign EVOLVE_WORKTREE_ROOT | 6/10 | `go test -run TestFlags` with the env set |
| env-hermetic predicate env | TestPredicateEnv_AllBranches passes with the env set | 6/10 | `go test -run TestPredicateEnv_AllBranches ./internal/acssuite/` |
| no foreign write | generate never touches the foreign root | 7/10 | `go test -tags acs ./acs/cycle2009` |
