# Comment history: `acs/cycle1279`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1279/predicates_test.go:3` — above `package cycle1279`

```text
// Package cycle1279 encodes the cycle-1279 acceptance criteria for
// `continuation-defect-ledger` (batch-integrity-review-2026-08-04.md F1 —
// defect laundering across salvage/continuation chains).
//
// Each predicate drives the REAL production seam through the package's own
// behavioral tests: `hooks.Classify` (the audit verdict path) for the ledger
// emit + disposition diff, and `faillearn.WriteArtifacts` (the function
// core.writeDeterministicLearning and cmd_loop_outcome already call) for the
// transactional inbox write. Nothing here greps source for a magic string — a
// predicate that passed on a string would pass on dead code, which is the
// exact failure mode this cycle exists to close.
//
// Shape discipline (flaky-predicate lint): every subprocess is ONE named
// package with a narrowed `-run`, `cmd.Dir` is set explicitly (never a bare
// cwd-relative git/go invocation, which resolves differently in a fleet lane),
// and the context bound comes from the test deadline rather than a literal
// wall-clock budget.
```
