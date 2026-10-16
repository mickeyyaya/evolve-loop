# Comment history: `internal/quotareset`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero K1

### `go/internal/quotareset/compute_param_test.go:19` — above `func refNow() time.Time { return time.Date(2026, 6, 20, 14, 0, 0, 0, time.UTC) }`

```text
// refNow is a deterministic clock: 2026-06-20 14:00:00 UTC. Hint times before
// 14:00 roll to tomorrow; times after stay today.
```
