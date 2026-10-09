---
score_cap:
  - criterion: "a watch whose run never completes exits 2 and names the last observed status on stderr"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1845_00[123]' ./acs/cycle1845"
  - criterion: "a completed green run still exits 0 and a completed red run exits 1"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run TestC1845_004 ./acs/cycle1845"
  - criterion: "a red workflow is never masked into exit 0 by a later hung workflow"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1845_005 ./acs/cycle1845"
  - criterion: "a gh call that never returns still ends the watch at its deadline with exit 2"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1845_006 ./acs/cycle1845"
---

# Eval: evolve ci watch timeout exit code

> Pins the exit contract of `evolve ci watch` after the PR #822 merge on 2026-10-09,
> where a timed-out watch ("last status in_progress") was read by its caller as green.
> The predicates drive the real binary with a fake gh on PATH.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| never-completes | exit 2 plus last status on stderr | 3/10 | `go test -run 'TestC1845_00[123]'` |
| green-red | exit 0 green, exit 1 red | 4/10 | `go test -run TestC1845_004` |
| red-not-masked | red plus hung never exits 0 | 6/10 | `go test -run TestC1845_005` |
| hung-gh | blocked gh call bounded by the deadline | 5/10 | `go test -run TestC1845_006` |
