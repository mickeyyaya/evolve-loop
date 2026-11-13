---
score_cap:
  - criterion: "A cancelled context ends the submit-verify re-send wait within one poll, outcome not_verified"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestVerifySubmitted_CancelDuringBackoffEndsTheWaitWithinOnePoll$' ./internal/bridge"
  - criterion: "Cancel during the 4th backoff step sends no further keys"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestVerifySubmitted_CancelDuringFourthBackoffStopsFurtherSendKeys$' ./internal/bridge"
  - criterion: "The nudge-site wait loop counts the re-send window in elapsed time"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestNudgeSite_ResendWindowCountsInElapsedWaitTime$' ./internal/bridge"
  - criterion: "Every new bridge test is parallel or listed in serialTests (the bridge suite gate stays green)"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial$' ./internal/bridge"
---

# Eval: submit-verify re-send wait stops on cancel

> Pins three properties of `verifySubmitted` and its nudge caller. A cancelled
> context must end the re-send backoff within one poll as not_verified. No
> SendKeys may run after the cancel. The re-send window must count in the wait
> loop's elapsed time. Source incident: cycle 2350 inbox item
> submit-verify-sleep-stops-on-cancel (the wait used `time.Sleep`, which ignores ctx).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cancel-latency | Wait ends within one poll as not_verified | 7/10 | `go test -run TestVerifySubmitted_CancelDuringBackoffEndsTheWaitWithinOnePoll` |
| no-send-after-cancel | 4th-step cancel stops SendKeys | 7/10 | `go test -run TestVerifySubmitted_CancelDuringFourthBackoffStopsFurtherSendKeys` |
| elapsed-accounting | Re-send window counted in elapsed | 6/10 | `go test -run TestNudgeSite_ResendWindowCountsInElapsedWaitTime` |
| bridge-suite-gate | New serial tests are listed in serialTests (audit round 1, H1) | 5/10 | `go test -run TestParallelDefault_EveryTopLevelTestIsParallelOrListedSerial` |
