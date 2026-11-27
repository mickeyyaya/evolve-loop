---
score_cap:
  - criterion: "A replay of wave 185 (0 ships, 8 live lanes, abnormal events of one kind) makes evolve wave watch print a stall event before the 120 minute no-ship age"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2664_001_' ./acs/cycle2664"
  - criterion: "Replays of wave 184 and earlier waves at normal volume print no stall event"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2664_002_' ./acs/cycle2664"
  - criterion: "The stall event names the trigger, the lanes and existing evidence paths, in JSON and in text"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2664_00(3|4)_' ./acs/cycle2664"
  - criterion: "One standing trigger prints one stall event across polls, and the bounds come from policy wave.checkin"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2664_00(5|6)_' ./acs/cycle2664"
  - criterion: "wave-operations.md names the zero-ship first look and passes the STE check"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2664_007_' ./acs/cycle2664"
---

# Eval: The wave watch wakes the console on a stall

> Pins the stall wake of `evolve wave watch`. Wave 185 ran 5.7 hours with 8 live lanes and 0 of 9 cycles shipped.
> LOOP_ORCH_LANE_STALLED fired 9 times and the abnormal event kind verify-single-flight-unavailable fired 75 times.
> No watch line woke the console on them, and the console read zero ships at about 10:00Z and waited until 13:50Z.
> Cycle 2599 built the first attempt and failed the build handoff on a test-audit row for an error-text match.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wake-185 | the wave 185 replay wakes the watch before 10:00Z | 6/10 | `go test -tags acs -run TestC2664_001_ ./acs/cycle2664` |
| silent-normal | wave 184 and earlier replays stay silent | 7/10 | `go test -tags acs -run TestC2664_002_ ./acs/cycle2664` |
| names-facts | the wake names trigger, lanes and evidence paths | 6/10 | `go test -tags acs -run TestC2664_003_ ./acs/cycle2664` |
| once-and-policy | one wake for one trigger, bounds from policy | 6/10 | `go test -tags acs -run TestC2664_005_ ./acs/cycle2664` |
| doc-first-look | the doc names the zero-ship first look | 5/10 | `go test -tags acs -run TestC2664_007_ ./acs/cycle2664` |
