---
score_cap:
  - criterion: "evolve status --json emits loop, cycles, streak, prs, ci and the loop section reflects lease and brake"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_(009|010)' ./acs/cycle1786"
  - criterion: "evolve status is read-only and registered in help"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_(011|012|014)' ./acs/cycle1786"
  - criterion: "the streak counts consecutive SHIPPED cycles back from the newest dossier, stops at the first non-ship, and names the last zero-ship run (JSON and human)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_01[56]' ./acs/cycle1786"
  - criterion: "prs carry each open PR's check state and ci names the failing jobs of main's latest required CI run"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_01[89]' ./acs/cycle1786"
  - criterion: "with gh missing or failing, prs and ci read unavailable and the exit is 0; an unreadable snapshot exits 2"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1786_02[01]' ./acs/cycle1786"
---

# Eval: evolve status

> Pins the read-only `evolve status [--json]` report (loop, cycles, streak, prs, ci), cycle 1786. The audit of the first build (round 1, H1 and M2) found the streak inverted (it counted trailing unshipped cycles, while the cli-status item asks for the consecutive-ship streak back from the newest dossier plus the last zero-ship run), PRs listed without check state, CI read as the latest run on any branch with no failing job names, and no exit 2 for an unreadable snapshot. The later rows pin those behaviors against dossier fixtures and a fake `gh`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| status-shape | five JSON sections, loop reflects lease/brake | 6/10 | `go test -tags acs -run TestC1786_(009|010) ./acs/cycle1786` |
| status-readonly | no tree mutation, listed in help | 7/10 | `go test -tags acs -run TestC1786_(011|012|014) ./acs/cycle1786` |
| ship-streak-direction | consecutive shipped newest-first, stop at first non-ship, last zero-ship run | 8/10 | `go test -tags acs -run TestC1786_01[56] ./acs/cycle1786` |
| remote-content | PR check state; failing jobs of main's latest required run | 7/10 | `go test -tags acs -run TestC1786_01[89] ./acs/cycle1786` |
| degrade-and-exit-codes | gh missing/failing → unavailable, exit 0; unreadable snapshot → exit 2 | 7/10 | `go test -tags acs -run TestC1786_02[01] ./acs/cycle1786` |
