# Comment history: `internal/cyclebudget`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d14

### `go/internal/cyclebudget/cyclebudget_test.go:21` — above `d := cyclebudget.Next(cyclebudget.Enforce, 2, 25, 0, false)`

```text
// Backlog drained after a real cycle ⇒ goal complete ⇒ stop (the cycle-4
// over-reach this feature prevents).
```
