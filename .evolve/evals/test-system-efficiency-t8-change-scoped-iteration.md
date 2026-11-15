---
score_cap:
  - criterion: "The selected set holds the changed package and each direct and transitive importer, and excludes unrelated packages"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2418_001|TestC2418_002|TestC2418_005' ./acs/cycle2418"
  - criterion: "A failing selected test exits 1 and an empty selection exits 0"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2418_003|TestC2418_004' ./acs/cycle2418"
  - criterion: "The helper is shadow only: it leaves the tree untouched and no production file besides the verb and its registration mentions it"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2418_007|TestC2418_008' ./acs/cycle2418"
  - criterion: "The full floor still lists every tier and no workflow or Makefile target runs the scoped verb"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2418_009|TestC2418_010' ./acs/cycle2418"
---

# Eval: Change-scoped shadow test helper (test-system plan T8)

> Pins the contract of `evolve test-scoped`, a lane helper that runs the tests of the packages a lane changed and of their importers. It is in shadow: no gate reads its result, and the full floor still runs before each pull request. Source: inbox item `test-system-efficiency-t8-change-scoped-iteration`, decision D16 of `docs/plans/test-system-efficiency-2026-10.md`, cycle 2418.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| importer-closure | Changed package plus each importer, nothing unrelated | 4/10 | `TestC2418_001`, `_002`, `_005` |
| exit-contract | Exit 1 on a failing selected test, exit 0 on an empty selection | 5/10 | `TestC2418_003`, `_004` |
| shadow-only | No state written, no gate source mentions the verb | 6/10 | `TestC2418_007`, `_008` |
| floor-intact | Floor tiers and Makefile default are unchanged | 6/10 | `TestC2418_009`, `_010` |
