---
score_cap:
  - criterion: "evolve inbox lint reports every undeclared dispatchable item with the protected basenames its text names, and never a guessed path"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1929_00[12]_' ./acs/cycle1929"
  - criterion: "evolve inbox lint is a listed inbox verb that exits 0 on a declared inbox and 10 on a bad argument"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1929_003_' ./acs/cycle1929"
  - criterion: "the three hand-typed records of 2026-09-26 are reported undeclared on their pre-curation records"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1929_004_' ./acs/cycle1929"
  - criterion: "the ADR-0072 halt writer and the retro filers write files[] or a pipeline-* kind on every item they file"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestInboxWriters_EveryAutofiledItemDeclaresSurface$' ./cmd/evolve"
  - criterion: "the protected fail-learning floor golden stays byte-identical and green"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestWriteFloor_WritesLessonAndReportByteIdenticalToTheGolden$' ./internal/core/failurelearning"
  - criterion: "evolve inbox lint counts only tracked files, at any depth, as protected basenames"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1929_00[89]_' ./acs/cycle1929"
---

# Eval: an inbox item declares its fix surface when it is filed

> This eval pins F33 (inbox item `inbox-filing-declares-surface`). The router routes on the surface a record states: a `pipeline-*` kind or a path-shaped `files[]` token (`inboxbatch.Item.DeclaredSurface`). An item that names a protected file only by basename stays lane-eligible. It then fails at triage. On 2026-09-26 the console hand-typed three such items before wave 7. Commit `a603c42f3` curated them. Source: cycle 1929 TDD contract, `go/acs/cycle1929/predicates_test.go`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| lint-reports | undeclared dispatchable items are reported with cited protected basenames, no guessed path | 8/10 | `go test -tags acs -run 'TestC1929_00[12]_' ./acs/cycle1929` |
| lint-verb | the verb is listed, exits 0 when clean and 10 on a bad argument | 5/10 | `go test -tags acs -run 'TestC1929_003_' ./acs/cycle1929` |
| replay | the three 2026-09-26 records are reported undeclared | 7/10 | `go test -tags acs -run 'TestC1929_004_' ./acs/cycle1929` |
| filers-declare | the halt and retro filers declare a surface | 7/10 | `go test -run '^TestInboxWriters_EveryAutofiledItemDeclaresSurface$' ./cmd/evolve` |
| floor-golden | the protected fail-learning floor golden stays green (audit round 1, H1) | 8/10 | `go test -run '^TestWriteFloor_WritesLessonAndReportByteIdenticalToTheGolden$' ./internal/core/failurelearning` |
| tracked-only | the lint reads tracked files only, nested `runs`/`inbox`/`worktrees` included (audit round 1, M1) | 6/10 | `go test -tags acs -run 'TestC1929_00[89]_' ./acs/cycle1929` |
