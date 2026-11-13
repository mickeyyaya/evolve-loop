---
score_cap:
  - criterion: "The research doc ranks the wall-variance components by share, with per-test evidence that exists in the repo"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2360_01[34]_' ./acs/cycle2360"
  - criterion: "Every ranked component names what it waits on, and the doc passes ste-lint"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2360_01[56]_' ./acs/cycle2360"
  - criterion: "Each fix of the doc has a filed inbox item with acceptance"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2360_017_' ./acs/cycle2360"
  - criterion: "The CPU judgment passes a slow noisy host, fails a real regression, and reports below three reference packages"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2360_0(0[1-9]|10)_' ./acs/cycle2360"
  - criterion: "Both CI ratchet steps as written fail a regression and no longer pass -ratchet-report-only"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2360_01[12]_' ./acs/cycle2360"
---

# Eval: Find which component makes the CI package wall time swing, then restore the hard ratchet

> Pins the four acceptance criteria of inbox item ci-wall-variance-deep-dive. Source incident: PR #1226 measured
> internal/bridge at 331 s and 748 s on the same code (runs 40582532714 and 40611567029), and the operator made the
> package wall ratchet report-only. The existing research (docs/research/ci-wall-variance-2026-11.md) ranks causes
> without a share and files only two of the nine fixes.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| doc-ranks-by-share | Share column sums to 100 and descends; cited tests exist | 5/10 | `go test -tags acs -run 'TestC2360_01[34]_' ./acs/cycle2360` |
| doc-waits-on-and-ste | Waits-on names CPU, process start, tmux, git or sleeps; ste-lint clean | 6/10 | `go test -tags acs -run 'TestC2360_01[56]_' ./acs/cycle2360` |
| fixes-filed | Each fix row names an inbox item that exists | 6/10 | `go test -tags acs -run 'TestC2360_017_' ./acs/cycle2360` |
| noise-robust-judgment | CPU divided by a median speed index, 1.35x and 15 s, three reference packages | 4/10 | `go test -tags acs -run 'TestC2360_0' ./acs/cycle2360` (predicates 001 to 010) |
| hard-fail-restored | Both workflow steps fail a regression with no report-only flag | 4/10 | `go test -tags acs -run 'TestC2360_01[12]_' ./acs/cycle2360` |
