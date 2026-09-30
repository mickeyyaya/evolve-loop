# Comment history: `internal/cycleoutcome`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/cycleoutcome/cycleoutcome.go:88` — above `committed := CommittedIDsFor(in.Workspace)`

```text
// Continuation/retry cycles carry NO triage-decision.json (the
// continuation path binds the task directly), so the committed set
// resolved nil and the durable failure_count was never bumped — after 15
// FAILs every live inbox item sat at 0 and the TaskRetryCeiling
// quarantine + deep-escalation governors were unreachable (2026-08-10
// investigation). The lane-scope pin is the worked set on those cycles.
// File-ABSENT, not merely empty (diff-review MEDIUM): a fresh cycle whose
// triage legitimately committed zero ids must not have its lane-scope menu
// blamed for the failure — the pinned items were explicitly declined.
```
