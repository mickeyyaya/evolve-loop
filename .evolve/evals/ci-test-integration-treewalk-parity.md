---
score_cap:
  - criterion: "make test-integration fails when a test leaves a path outside the allow list"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -tags integration -run '^(TestMakeIntegration_FailsWhenATestLeaksAPathOutsideTheAllowList|TestMakeIntegration_FailsOnALeakEvenWhenATestAlsoFails)$' ./internal/ciparity"
  - criterion: "make test-integration passes on a clean tree with its coverage, bin and rusage outputs present"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -tags integration -run '^(TestMakeIntegration_PassesOnACleanTreeWithItsOwnOutputsPresent|TestMakeIntegration_PassesWithTheDefaultOutputsOnly|TestMakeIntegration_IgnoresPathsThatExistedBeforeTheRun|TestMakeIntegration_KeepsTheTestFailureWhenNothingLeaks)$' ./internal/ciparity"
  - criterion: "treewalk check --allow skips only whole-component prefixes and rejects an empty prefix"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestFilterDropsPathsAtOrUnderAnAllowedPrefix|TestFilterKeepsOrderAndLeavesItsInputsUntouched|TestFilterAppliedToADiffHidesOnlyTheAllowedLeaks)$' ./internal/treewalk && go test -count=1 -run '^(TestRunTreewalkCheckAllowSkipsOnlyTheListedPrefixes|TestRunTreewalkCheckRejectsAnEmptyAllowEntry)$' ./cmd/evolve"
  - criterion: "make test keeps its treewalk snapshot and check"
    max_if_missing: 6
    evidence: "cd go && make -n test | grep -q 'treewalk snapshot' && make -n test | grep -q 'treewalk check'"
---

# Eval: CI test-integration runs the treewalk leak guard that make test runs

> Pins the parity between `make test` and `make test-integration`. Cycle 2419 (wave 174)
> showed the gap: three tests leaked `.evolve` ledgers under `go/`, CI on main stayed green
> because only `make test` had the treewalk guard, and every lane composed gate went red
> (`ORCHESTRATOR_COMPOSED_GATE_DECLINED`, 4 LEAK paths). The tests run the real
> `go/Makefile` recipe in a throwaway module, so a leak that the target ignores fails them.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| leak-fails-target | a leaked path outside the allow list fails the target | 3/10 | `go test -tags integration -run TestMakeIntegration_FailsWhenATestLeaksAPathOutsideTheAllowList ./internal/ciparity` |
| clean-passes-with-outputs | the target writes only allowed outputs on a clean tree | 4/10 | `go test -tags integration -run TestMakeIntegration_PassesOnACleanTreeWithItsOwnOutputsPresent ./internal/ciparity` |
| allow-list-semantics | prefixes match whole path components, an empty prefix is refused | 5/10 | `go test -run TestFilterDropsPathsAtOrUnderAnAllowedPrefix ./internal/treewalk` |
| fast-target-unchanged | `make test` keeps its guard | 6/10 | `make -n test` shows `treewalk snapshot` and `treewalk check` |
