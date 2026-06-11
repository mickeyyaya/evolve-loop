---
score_cap:
  - criterion: "a worker still queued on the bounded semaphore observes context.Canceled when a sibling's fatal failure cancels the root context"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestDispatch_CancelWhileQueuedOnSemaphore ./internal/swarm/"
  - criterion: "Dispatch function statement coverage stays at or above 97%"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -coverprofile=/tmp/swarm-dispatch-eval.out ./internal/swarm/ >/dev/null 2>&1 && go tool cover -func=/tmp/swarm-dispatch-eval.out | awk '$2==\"Dispatch\"{gsub(/%/,\"\",$3); exit !($3>=97)}'"
---

# Eval: Dispatch semaphore-cancel arm coverage

> Pins coverage of the `case <-rootCtx.Done()` arm of `Dispatch`
> (`go/internal/swarm/dispatcher.go`), introduced/covered in cycle 294. When the
> bounded-concurrency semaphore is saturated (Concurrency < workers) and one
> worker's fatal launch failure cancels the derived root context, a sibling
> goroutine still blocked on `sem <- struct{}{}` must fall through to
> `case <-rootCtx.Done()` and record `WorkerResult{..., Err: rootCtx.Err()}`
> rather than hang. Before this cycle that arm was uncovered (Dispatch func =
> 96.0%): no test drove cap < workers with an early fatal failure. The cycle-294
> test drives 3 workers at Concurrency:1 with a failing+slow w0 so a queued worker
> observes `context.Canceled`, proving structured-concurrency teardown reaps every
> worker (no hung goroutines).
> Source incident: cycle 294 (scout-report.md T2 swarm-dispatch-semaphore-cancel).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| queued-worker-cancel | a semaphore-queued worker gets context.Canceled on root-context cancel | 7/10 | `go test -run TestDispatch_CancelWhileQueuedOnSemaphore ./internal/swarm/` |
| dispatch-coverage-floor | Dispatch function coverage >= 97% | 5/10 | `go tool cover -func ... Dispatch >= 97%` |
