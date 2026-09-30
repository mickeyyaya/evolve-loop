# Comment history: `acs/cycle1525`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1525/predicates_test.go:3` — above `package cycle1525`

```text
// Package cycle1525 materializes the cycle-1525 acceptance criterion for the
// fleet-scoped task `cap-audit-report-length`: doc/code drift protection for
// the whole-report size budget introduced in the cycle-1522 salvage
// (audit_report_length_test.go / audit.go:auditReportMaxBytes).
//
// Everything else this task's eval pins (cap-exists, overflow-recorded,
// non-lossy, no-regression — .evolve/evals/cap-audit-report-length.md) is
// already covered, live and GREEN, by
// go/internal/phases/audit/audit_report_length_test.go's TestAuditReportLength
// table (verified pre-existing GREEN this cycle, see test-report.md). The one
// AC that eval pins but no test file yet materializes is criterion 5,
// "doc-code-sync": agents/evolve-auditor-reference.md must document the SAME
// numeric budget the gate enforces (auditReportMaxBytes in
// internal/phases/audit/audit.go). This predicate is that missing piece.
```

### `go/acs/cycle1525/predicates_test.go:29` — above `func TestC1525_001_DocCapMatchesCodeCap(t *testing.T) {`

```text
// TestC1525_001_DocCapMatchesCodeCap cross-references the numeric budget
// documented in agents/evolve-auditor-reference.md against the live
// auditReportMaxBytes constant in internal/phases/audit/audit.go. It is a
// static-text comparison, not a subprocess/behavioral call — there is no
// runtime seam to drive for "does the doc match the code" — so it is declared
// as a waived config-check predicate (cycle-85 classification table) rather
// than dressed up as a fake behavioral test. It still FAILS the moment either
// side drifts, which is the load-bearing property: bumping the code constant
// without updating the doc (or vice versa) breaks this predicate immediately.
//
// acs-predicate: config-check
```
