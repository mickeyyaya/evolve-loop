# Comment history: `internal/paralleltest`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d14

### `go/internal/paralleltest/doc.go:1` — above `package paralleltest`

```text
// Package paralleltest is the one parallel-by-default guard of the package test trees.
// Each package test reads its own sources and lists its serial tests; the guard flags a test that is
// neither parallel nor listed, and a parallel test that changes process state directly, through a
// helper or through a method on a package-level value.
// See docs/explain/builds/cycle-1869-01m4rvyfk91nmt5wksrgbwsq88.md.
```
