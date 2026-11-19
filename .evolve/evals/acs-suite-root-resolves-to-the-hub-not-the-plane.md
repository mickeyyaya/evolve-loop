---
score_cap:
  - criterion: "evolve acs suite from a hub-layout cycle worktree resolves the plane that holds the cycle state, not the hub"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2528_001|TestC2528_003' ./acs/cycle2528"
  - criterion: "the suite reads the plane's .evolve/policy.json"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2528_002 ./acs/cycle2528"
  - criterion: "with no worktree holding the cycle state the suite fails naming the cycle and the roots checked"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC2528_004 ./acs/cycle2528"
  - criterion: "suiteProjectRoot resolves the plane through git worktree list and keeps the no-cycle derivation"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestSuiteProjectRoot' ./cmd/evolve"
---

# Eval: acs suite root resolves to the plane, not the hub

> Pins the fix for the hub layout: a bare common dir with a plane worktree and a cycle worktree under it. `suiteProjectRoot` returned the parent of the git common dir (the hub), so the suite read a stale policy. Source incident: cycle 2528 inbox item.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| plane-not-hub | project_root is the plane | 8/10 | `go test -tags acs -run 'TestC2528_001\|TestC2528_003' ./acs/cycle2528` |
| plane-policy | plane policy.json is read | 7/10 | `go test -tags acs -run TestC2528_002 ./acs/cycle2528` |
| missing-state | hard failure names cycle and roots | 7/10 | `go test -tags acs -run TestC2528_004 ./acs/cycle2528` |
| unit | suiteProjectRoot hub and no-cycle cases | 7/10 | `go test -run TestSuiteProjectRoot ./cmd/evolve` |
