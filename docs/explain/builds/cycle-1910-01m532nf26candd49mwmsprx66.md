# Build Explanation — Cycle 1910

## Build Binding
- Cycle: 1910
- Base SHA: de856064796ef2afb3de94abcc8d50e6f5b5e8f8

## Summary
A put blocked on a full best-effort queue now returns when the queue closes. Before, it waited the full enqueue deadline.

## Rationale
`closeQueue` set `closed` but never signalled `space`, so a blocked `push` slept until its deadline. The `space` channel has one slot, so one signal wakes one put. `closeQueue` sends the first signal. Each woken put sees the closed queue and sends the signal on, so every blocked put returns. A separate close channel was rejected: it adds a field and a select case for the same result.

## Changed Areas
- `go/internal/events/publisher/queue.go` — `closeQueue` and the closed branch of `push` signal `space`.
- `go/internal/events/publisher/queue_test.go` — tests for one blocked put, several blocked puts and a put after close.
- `docs/architecture/packages/internal-events-publisher.md` — states the close signal in the best-effort queue description.
- `go/acs/cycle1910/predicates_test.go` — the TDD phase predicates for the three behaviours.
- `.evolve/evals/publisher-close-notifies-space.md` — the eval graders for the task.

## Design Decisions
The wake is chained, not broadcast, so the one-slot channel and its other senders stay unchanged.

## Verification
`go test -count=1 -race ./internal/events/publisher/` passes. `evolve acs suite --cycle 1910` reports green=170 red=0.

## Compatibility
No exported symbol, flag or schema changes. A put after close still returns `queue_full` at once.

## Limitations
The chain wakes puts one after another; the cost is one channel send per blocked put.
