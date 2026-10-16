---
score_cap:
  - criterion: "a put blocked on a full queue returns at close"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -race -run '^TestPublisher_ABlockedPutReturnsAtClose$' ./internal/events/publisher/"
  - criterion: "every put blocked on a full queue returns at close, not only one"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -race -run '^TestPublisher_EveryBlockedPutReturnsAtClose$' ./internal/events/publisher/"
  - criterion: "a put after close is refused without waiting the enqueue deadline"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -race -run '^TestPublisher_APutAfterCloseIsRefusedWithoutWaiting$' ./internal/events/publisher/"
  - criterion: "the publisher package stays green"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -race ./internal/events/publisher/"
---

# Eval: closeQueue wakes puts blocked on space

> `closeQueue` in `go/internal/events/publisher/queue.go` sets `closed` but never signals `q.space`, so a `push` blocked in its `select` waits the full enqueue deadline. Source: E6 delta review, PR #828, cycle 1910.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| positive | one blocked put returns at close with queue_full | 3/10 | `go test -run ^TestPublisher_ABlockedPutReturnsAtClose$` |
| edge | several blocked puts all return (space is a 1-slot channel) | 5/10 | `go test -run ^TestPublisher_EveryBlockedPutReturnsAtClose$` |
| negative | a put after close is refused at once | 6/10 | `go test -run ^TestPublisher_APutAfterCloseIsRefusedWithoutWaiting$` |
| regression | package stays green | 5/10 | `go test -race ./internal/events/publisher/` |
