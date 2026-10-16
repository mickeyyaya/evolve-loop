# Comment history: `internal/codequality`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d12

### `go/internal/codequality/gofmt.go:1` — above `package codequality`

```text
// Package codequality runs deterministic source-format gates that mirror the
// CI `vet + fmt` step, so the autonomous cycle's audit can catch gofmt
// regressions before ship instead of leaking them to CI. Cycles 339-341
// shipped to main CI-red because the cycle-scoped audit never ran gofmt over
// the generated go/acs/cycle<N>/*.go predicate files; this package closes that
// gap in-process.
```
