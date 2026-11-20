---
score_cap:
  - criterion: "A claude pane that ignores Enter for 20 s and then accepts it ends submitted_after_resend"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestVerifySubmitted_EnterIgnoredFor20sThenAcceptedEndsSubmittedAfterResend$' ./internal/bridge"
  - criterion: "The submit resend schedule is a table fed from Deps and a pane that never accepts Enter still ends submit_wedged"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestVerifySubmitted_(DefaultScheduleResendsAtTwoFiveTenTwentyThirtySeconds|ResendScheduleComesFromDeps)$' ./internal/bridge"
  - criterion: "Four concurrent seats each submit after a late Enter, three rounds in a row"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestVerifySubmitted_FourConcurrentSeatsAllSubmitThreeRoundsInARow$' ./internal/bridge"
  - criterion: "The last scheduled Enter is observed for a settle window before the seat is called wedged, and an accepted Enter is seen before the next one is sent"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestVerifySubmitted_(FinalScheduledEnterIsObservedBeforeGivingUp|AcceptedEnterIsObservedBeforeTheNextEnterIsSent|NeverAcceptedPaneIsObservedAfterTheLastEnterThenWedges)$' ./internal/bridge"
---

# Eval: A pasted prompt submits reliably when several claude seats boot together

> Pins the late-Enter fix for the claude-tmux paste wedge: the resend schedule moves from a hardcoded array to a Deps table (the policy key and its adapter wiring are a console follow-up), and a late accepted Enter ends submitted_after_resend. Source incident: inbox item claude-2316-large-paste-submit-wedge (cycle 2090 ops watcher), cycle 2550 TDD.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| late-enter | Enter accepted after 20 s submits | 4/10 | `TestVerifySubmitted_EnterIgnoredFor20sThenAcceptedEndsSubmittedAfterResend` |
| schedule-table | schedule from Deps, negative still wedges | 5/10 | `TestVerifySubmitted_ResendScheduleComesFromDeps` |
| final-observation | last Enter observed after a render latency; accepted Enter seen before the next | 5/10 | `TestVerifySubmitted_FinalScheduledEnterIsObservedBeforeGivingUp` |
| four-seats | 4 concurrent seats, 3 rounds, all submit | 6/10 | `TestVerifySubmitted_FourConcurrentSeatsAllSubmitThreeRoundsInARow` |
