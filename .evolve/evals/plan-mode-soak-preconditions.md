---
score_cap:
  - criterion: "codex plan_question rule answers five-option and narrow-pane wrapped pickers"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestCodexPlanRuleTailHeadroom_MatchesFiveOptionAndWrappedForms ./internal/bridge"
  - criterion: "plan_question tail_lines leaves at least two rows of slack over the largest fixture picker"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestCodexPlanRuleTailHeadroom_WindowLeavesSlackOverTheLargestForm ./internal/bridge"
  - criterion: "an over-long picker stays unmatched and the window stays inside (0, 30]"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCodexPlanRuleTailHeadroom_(OverLongFormIsUnmatched|WindowStaysBounded)' ./internal/bridge"
  - criterion: "answered pickers do not re-fire once the window widens"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestCodexPlanRuleTailHeadroom_AnsweredFormsStayUnmatched|TestAutoRespond_PlanMode' ./internal/bridge"
---

# Eval: size the codex plan_question tail_lines headroom

> Pins the synthetic part (a) of plan-mode-soak-preconditions. tail_lines 9 gave one row of headroom over an 8-row picker, so a five-option or wrapped question moved the anchor out of the window and the rule went silently inert. The live codex capture, the effective-effort assertion, the /plan entry plumbing and the phase budget stay console work. Source incident: docs/incidents/2026-08-27-plan-mode-dialog-blind-spot.md, cycle 2104.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wrapped-five-option | rule fires on 10 and 11 row pickers | 4/10 | `go test -run TestCodexPlanRuleTailHeadroom_MatchesFiveOptionAndWrappedForms` |
| slack | tail_lines minus largest picker is at least 2 | 5/10 | `go test -run TestCodexPlanRuleTailHeadroom_WindowLeavesSlackOverTheLargestForm` |
| bounded | over-long picker unmatched, window at most 30 | 6/10 | `go test -run TestCodexPlanRuleTailHeadroom_OverLongFormIsUnmatched` |
| answered | answered pickers stay unmatched | 6/10 | `go test -run TestCodexPlanRuleTailHeadroom_AnsweredFormsStayUnmatched` |
