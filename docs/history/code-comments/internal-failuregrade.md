# Comment history: `internal/failuregrade`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/failuregrade/failuregrade.go:1` — above `package failuregrade`

```text
// Package failuregrade is the ADR-0048 Slice A graduated-enforcement classifier.
//
// A gate/cycle failure is not binary (proceed | abort). It is graded into a
// TIER, so a tiny, provably-recoverable defect is corrected, quarantined, or
// repaired instead of costing a whole cycle (the cycles 243/246/247 killers): a
// missing challenge token in a report (a 60-second re-emit), benign config-file
// tree-churn at the tree-diff guard, or a SELF_SHA mismatch after a verified
// reproducible rebuild.
//
// The vocabulary is CLOSED (ADR-0047 Specification pattern): only three named,
// evidence-anchored failure classes grade below Abort, and a class missing its
// evidence predicate still Aborts. So the failure floor (ADR-0039) can never
// silently downgrade a real failure — an unknown reason, or a tree-churn that is
// NOT benign, or a SELF_SHA mismatch that is NOT a verified rebuild (i.e. real
// tampering), all still abort the cycle.
//
// The matched signatures are OUR OWN control reason-codes, never agent content
// — substring matching here is safe precisely because the input is the
// orchestrator's abort reason, not a captured pane (contrast ADR-0047).
```

### `go/internal/failuregrade/failuregrade.go:31` — above `TierCorrect`

```text
// TierCorrect re-dispatches the phase with a correction directive (the
// ADR-0045 mechanism) instead of failing — e.g. "re-emit the report with
// challenge token <T>".
```
