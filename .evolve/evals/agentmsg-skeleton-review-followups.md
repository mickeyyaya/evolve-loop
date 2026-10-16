---
score_cap:
  - criterion: "An Ack or a Read by one dispatch leaves the queue of another dispatch unchanged"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestAck_CrossQueueIndependence$' ./internal/agentmsg/broker"
  - criterion: "A gap emits one msg.dead event per distinct range on re-read and stays in every mailbox"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRead_(GapEventsAreDeduplicatedOnReRead|AGapFromTheStoreIsReportedAndNeverSilent)$' ./internal/agentmsg/broker"
  - criterion: "Spec §4 seal rule names planned entries, matching §10"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1888_003_' ./acs/cycle1888"
  - criterion: "Spec §3 and §10 document at-least-once redelivery of a planned-phase message to a retry"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1888_004_' ./acs/cycle1888"
  - criterion: "Spec §6 step 7 and T12 say that a read from the head clears the queue"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1888_005_' ./acs/cycle1888"
  - criterion: "The broker package stays green and its changed files carry no comments"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1888_00(6|7)_' ./acs/cycle1888"
---

# Eval: Agent messaging skeleton review follow-ups

> Pins the ADR-0129 review follow-ups (inbox item of 2026-10-11) before M2.
> The broker emitted one gap `msg.dead` event on every re-read (cycle 1888 RED: four events for one gap over four reads).
> The spec disagreed with itself on the seal (§4 versus §10), claimed one copy for a retried phase, and claimed a caller cursor never empties a queue.
> Source incident: cycle 1888 TDD phase.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| cross-queue | Ack/Read by X leaves Y's queue | 7/10 | `TestAck_CrossQueueIndependence` |
| gap-dedup | one event per distinct gap, gap never silent | 8/10 | `TestRead_GapEventsAreDeduplicatedOnReRead` |
| seal-spec | §4 names planned entries | 6/10 | `TestC1888_003_` |
| at-least-once | §3/§10 document retry redelivery | 6/10 | `TestC1888_004_` |
| head-read | §6 step 7 and T12 | 6/10 | `TestC1888_005_` |
| regression | package green, zero comments | 8/10 | `TestC1888_006_`, `TestC1888_007_` |
