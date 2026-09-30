# Comment history: `acs/cycle1282`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1282/predicates_test.go:3` — above `package cycle1282`

```text
// Package cycle1282 encodes the cycle-1282 acceptance criteria for
// `continuation-defect-ledger`. Cycle 1282 is the CONTINUATION of cycle 1279:
// the anti-laundering ledger landed there and its own audit rejected it with
// seven defects (.evolve/runs/cycle-1279/audit-report.md, D1–D7). These
// predicates grade the disposition of that defect list — which is, fittingly,
// exactly the property the mechanism exists to enforce.
//
// Every predicate drives the REAL production seam through the owning package's
// behavioral tests — `hooks.Classify` (the audit verdict path),
// `Orchestrator.writeDeterministicLearning` (the failure floor the production
// path calls at failure_learning.go:366/372), and `faillearn.WriteArtifacts`.
// Nothing here greps source for a magic string: a predicate that passed on a
// string would pass on dead code, which is the failure mode this cycle exists
// to close.
//
// Shape discipline (flaky-predicate lint): every subprocess is ONE named
// package with a narrowed `-run` (./internal/core is a known 40s+ suite and is
// never swept whole), `cmd.Dir` is set explicitly rather than inherited from a
// fleet lane's cwd, and the bound is a process-reaping timeout, not a
// performance assertion.
```
