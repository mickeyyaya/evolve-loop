# Comment history: `internal/dispatchevents`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d8

### `go/internal/dispatchevents/events.go:1` — above `package dispatchevents`

```text
// Package dispatchevents writes abnormal-events.jsonl entries from the
// dispatcher level — counter-non-advance, circuit-breaker trips, verify
// failures, classifier verdicts. Port of the inline JSONL emission at
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:983-993 plus the
// post-classifier records added in cycle-61 follow-ups.
//
// Distinct from internal/adapters/observer/, which watches per-phase
// stdout for stalls. dispatchevents emits at the *cycle* boundary;
// observer emits within a phase. Both write JSONL, but to different
// files and consumed by different audiences.
```
