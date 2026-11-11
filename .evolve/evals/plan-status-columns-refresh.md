---
score_cap:
  - criterion: "Each named status cell of the six plan docs cites a commit that exists on the branch or a PR"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2265_00[1-7]' ./acs/cycle2265"
  - criterion: "All six plan docs change and no decision or fact row is removed"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2265_008' ./acs/cycle2265"
  - criterion: "evolve docs ste-lint --strict reports 0 findings on the added lines of the changed plans"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2265_009' ./acs/cycle2265"
---

# Eval: plan docs status columns match main

> Pins the status cells of six plan docs under `docs/plans/` against the commits and pull requests that prove them. The 2026-11-03 plan audit found text that disagrees with main (agent-messaging M4, convergence V0, V3b and V4, logging H1 and H2, model-level-benches steps 1 and 2, retry-ladder `audit_fix`, test-system-efficiency T6). Source incident: cycle 2265 inbox item `plan-status-columns-refresh`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cited-status | Each status cell cites a real commit or PR | 5/10 | `go test -tags acs -run 'TestC2265_00[1-7]' ./acs/cycle2265` |
| decision-text-untouched | Six files change, no decision row removed | 6/10 | `go test -tags acs -run TestC2265_008 ./acs/cycle2265` |
| ste-lint | Added lines pass the STE lint | 7/10 | `go test -tags acs -run TestC2265_009 ./acs/cycle2265` |
