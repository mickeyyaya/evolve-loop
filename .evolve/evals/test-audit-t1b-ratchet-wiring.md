---
score_cap:
  - criterion: "evolve test audit --rebaseline <package> writes that package's shard, and a second run with one more finding refuses and names the finding"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2523_00[1-4]' ./acs/cycle2523"
  - criterion: "a rebaseline refuses any row the old baseline lacks: an equal-count swap (local or central) and growth from zero rows in a central package refuse and name the new finding, and leave the baseline unchanged"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2523_00[89]|TestC2523_010' ./acs/cycle2523 && go test -count=1 -run 'TestRebaseline_' ./internal/testaudit"
  - criterion: "a new weak test in any package, including a protected package and a foreign predicate package, fails the tree test and names the package, the test and the rule; deleting it passes"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2523_005' ./acs/cycle2523"
  - criterion: "a baseline row whose test is gone, including one in a package with no test file, fails the tree test and names the row"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2523_006' ./acs/cycle2523"
  - criterion: "the shards of cmd/evolve and internal/core exist at HEAD, are not git-ignored and match the tree"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2523_007' ./acs/cycle2523"
---

# Eval: wire the test-audit ratchet (T1b)

> Pins the wiring of `internal/testaudit` from plan
> `docs/plans/test-quality-audit-2026-11.md` decision D2: the verb
> `evolve test audit --rebaseline <package>`, the per-package shards and the tree test
> that fails on a finding with no row and on a row with no finding. Source: cycle 2523,
> inbox item test-audit-t1b-ratchet-wiring. Before this item no caller ran the ratchet,
> so tautology items T6c and T7 had no shard to read.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| verb | rebaseline writes, refuses growth, shrinks, rejects a missing package | 4/10 | `go test -tags acs -run 'TestC2523_00[1-4]' ./acs/cycle2523` |
| row-set | a rebaseline refuses every new row, not only a larger row count (audit round 1, H1 and M1) | 4/10 | `go test -tags acs -run TestC2523_008, 009 and 010 ./acs/cycle2523` |
| any-package | a new weak test fails the tree test in every kind of package | 4/10 | `go test -tags acs -run TestC2523_005 ./acs/cycle2523` |
| stale-row | a row with no test fails the tree test | 5/10 | `go test -tags acs -run TestC2523_006 ./acs/cycle2523` |
| shards-at-head | T6c and T7 find their shards | 6/10 | `go test -tags acs -run TestC2523_007 ./acs/cycle2523` |
