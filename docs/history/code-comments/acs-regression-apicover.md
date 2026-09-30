# Comment history: `acs/regression/apicover`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/regression/apicover/predicates_test.go:3` — above `package apicover`

```text
// Package apicover is the ADR-0050 Phase 5 ship-gate predicate. It enforces the
// COMPLETENESS half of the public-API coverage invariant — every ./internal/...
// package must appear in the go/.apicover-enforce SSOT, so a newly-added package
// cannot silently escape the apicover gate. The CORRECTNESS half (each enforced
// package is actually apicover-clean: 0 uncovered / 0 false-green) is hard-gated
// by .github/workflows/go.yml's "api-coverage enforce" step, which generates the
// integration coverage profile a per-cycle ship-gate predicate cannot afford to
// re-run. Together they keep the gate COMPLETE (this predicate) and CORRECT (CI).
```
