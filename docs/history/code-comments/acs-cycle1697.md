# Comment history: `acs/cycle1697`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1697/predicates_test.go:3` — above `package cycle1697`

```text
// Package cycle1697 materialises the acceptance criteria of the one
// fleet-scoped task pinned to this lane: `dead-red-acs-corpus-cleanup`
// (.evolve/inbox/processing/cycle-1697/2026-08-04T05-03-00Z-dead-red-acs-corpus-cleanup.json;
// triage top_n: delete go/acs/cycle1257 + go/acs/cycle1259 predicate files and
// extend F5 of docs/operations/batch-integrity-review-2026-08-04.md in place).
//
// THE DEFECT. fcdd466e shipped go/acs/cycle1257/predicates_test.go and
// go/acs/cycle1259/predicates_test.go from cycles whose audits FAILED. They
// grade an abandoned acssuite-internal selection design (the phantom
// GoLaneSelection and Run-stage unit tests) that never existed at any commit —
// the token is never spelled literally in this file, so 001's corpus scan
// covers this package too — so they are red by
// construction: at this cycle's base f341bc89, `go test -tags acs` reports 5
// FAIL in cycle1257 and 2 FAIL in cycle1259 — FAIL, not SKIP.
//
// MEASURED PREMISE CORRECTION. The inbox asks to "verify the EGPS
// skipped_count drops". It cannot: EGPS (acssuite.goLanePatterns,
// go/internal/acssuite/acssuite.go:379-395) runs only the current cycle, the
// regression set and redteam — never a historical cycle dir — and the two
// packages FAIL rather than SKIP. skipped_count is invariant under this change
// by construction, so a predicate demanding a drop would be permanently RED
// (the cycle-644 unsatisfiable-AC shape). The honest measurable is that the
// dead packages no longer resolve at all (001); the F5 closure must record the
// skipped_count outcome (002), and the Auditor checks it is recorded truthfully.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - POSITIVE : 001 — the Go toolchain no longer resolves either dead package
//     (directory gone / no Go files). A tombstone, a skip-guarded shell or a
//     renamed copy still lists test functions and FAILS; a package that fails
//     to compile is never read as "removed".
//   - DOCS     : 002 — F5 extended in place: exactly one F5 heading, F6 still
//     next, Issue/Gap/Solution intact, and a Closure block citing both deleted
//     paths, the closing cycle and the skipped_count outcome (config-check).
//   - NEGATIVE : 003 — the change set deletes both dead files, touches no other
//     go/acs file (over-deletion / collateral edits FAIL), touches no protected
//     control-plane surface (guards.IsProtectedSurface, the production SSOT),
//     and is non-vacuous.
```
