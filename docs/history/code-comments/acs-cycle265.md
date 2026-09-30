# Comment history: `acs/cycle265`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle265/predicates_test.go:3` — above `package cycle265`

```text
// Package cycle265 materializes the cycle-265 acceptance criteria for the
// `routingtest-coverage` task: push `internal/routingtest` package coverage from
// 23% to >=70% by exercising the 8 kernel-floor invariants, the
// RunAll/runPure/buildConfig engine pipeline, and additional Brick functions.
//
// These predicates are BEHAVIORAL (cycle-85 lesson): each one RUNS the
// system-under-test — here, the `routingtest` Go suite — as a subprocess and
// asserts on its real `go test -cover -v` output (coverage %, top-level PASS
// counts, absence of FAIL). None greps a source file. If the builder deletes a
// test, coverage and the PASS counts drop and these predicates fail. The
// builder's job is test files only; production code is out of scope.
```
