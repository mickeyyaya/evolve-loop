# Comment history: `internal/quotastate`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero k2

### `go/internal/quotastate/quotastate_test.go:10` — above `func refNow(t *testing.T) time.Time {`

```text
// refNow anchors relative reset times deterministically: 2026-07-03 in
// Asia/Taipei (the tz the golden pane was captured in), before both the 4:10pm
// session reset and the Jul 5 weekly reset.
```
