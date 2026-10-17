---
score_cap:
  - criterion: "treewalk reports a new empty directory that git status cannot show"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1936_002' ./acs/cycle1936"
  - criterion: "evolve treewalk check fails on a leaked path and passes on a clean tree"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1936_005|TestC1936_006' ./acs/cycle1936"
  - criterion: "make test runs the treewalk snapshot before and the check after go test"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1936_007' ./acs/cycle1936"
---

# Eval: walk-delta guard for package-test leaks

> Pins the walk-delta guard of the tests-leak-artifacts-into-package-dirs inbox item. A before/after filesystem walk, empty directories included and no allowlist, must run through the evolve CLI and make. Source incident: the empty .evolve/runs/cycle-0 and cycle-1 directories that git status never showed (cycle 1936).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| empty-dir | New empty dir appears in Diff | 4/10 | `go test -run TestC1936_002` |
| cli | check exits non-zero on a leak, zero when clean | 5/10 | `go test -run TestC1936_005` |
| make | test target snapshots, tests, checks | 6/10 | `go test -run TestC1936_007` |
