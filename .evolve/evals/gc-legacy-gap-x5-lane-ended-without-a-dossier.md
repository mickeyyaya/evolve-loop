---
score_cap:
  - criterion: "The dirty lane of an old cycle with no run directory is saved and removed"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2250_001' ./acs/cycle2250"
  - criterion: "A quota-paused lane stays while an ended sibling is released"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2250_002' ./acs/cycle2250"
  - criterion: "A continuation-bound lane stays while an unbound old lane is released"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2250_003' ./acs/cycle2250"
  - criterion: "A lane younger than salvage_after_hours stays, and zero means never"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2250_004' ./acs/cycle2250"
  - criterion: "A live lane of an old cycle is never planned"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2250_005' ./acs/cycle2250"
  - criterion: "Each changed function of go/internal/gc has 100% coverage"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2250_006' ./acs/cycle2250"
---

# Eval: gc X5 — a lane has ended without a dossier

> Pins decision D6 and component X5 of docs/plans/gc-closes-the-legacy-gap-2026-11.md. A lane
> is ended when it is not live, not paused, not bound to a continuation and older than
> salvage_after_hours; a close-out dossier is not needed. Source incident: the 2026-11-09
> dry run released 0 of 261 worktrees because a lane with no run directory had no dossier.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| ended-lane | Old dirty lane, no run directory, is salvaged and removed | 4/10 | `go test -tags acs -run TestC2250_001 ./acs/cycle2250` |
| paused-keep | Quota-paused lane stays, ended sibling goes | 4/10 | `go test -tags acs -run TestC2250_002 ./acs/cycle2250` |
| continuation-keep | Continuation-bound lane stays, unbound sibling goes | 4/10 | `go test -tags acs -run TestC2250_003 ./acs/cycle2250` |
| age-boundary | Young lane stays; zero salvage age means never | 5/10 | `go test -tags acs -run TestC2250_004 ./acs/cycle2250` |
| live-never | Live lane is never planned | 5/10 | `go test -tags acs -run TestC2250_005 ./acs/cycle2250` |
| coverage | 100% coverage of each changed function | 6/10 | `go test -tags acs -run TestC2250_006 ./acs/cycle2250` |
