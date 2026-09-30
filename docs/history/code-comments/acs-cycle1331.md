# Comment history: `acs/cycle1331`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1331/predicates_test.go:3` — above `package cycle1331`

```text
// Package cycle1331 materialises the cycle-1331 acceptance criteria for this
// fleet lane's two assigned todos (fleet_scope: audit-warn-prescription-gate,
// percycle-audit-apicover-newexport-parity).
//
// Investigation (tdd phase, this cycle) found BOTH todos already have their
// production fix committed at HEAD (git log: "052ee69f salvage snapshot
// (ADR-0076 continuation-on-fail)") — a prior lane's salvage landed
// phasecontract.Failure.Prescription + emitDefectLedger's prescription sourcing
// + a full white-box test suite (defect_ledger_prescription_test.go) BEFORE
// this lane started. Task 1's ACs are therefore pre-existing GREEN, not RED —
// see test-report.md for the full "cannot manufacture RED for already-shipped
// work" reasoning. Task 2's untested edge (scout Finding 4: a new exported
// symbol landing in an EXISTING enforced package via a brand-new file) had NO
// regression test anywhere in the tree; this cycle adds one
// (ciparity_newexport_test.go) that also passes immediately, confirming the
// scout's Hypothesis 2 ("the code path is correct in the common case; the risk
// is an untested edge") rather than surfacing a defect.
//
// Predicate strategy — each predicate shells `go test -run <pattern> -count=1
// -v <pkg>` over the DEFAULT (non-acs) suite and requires a `--- PASS: <name>`
// line per named test (the cycle-997/cycle-1329 SubprocessOutput precedent).
// Asserting on the PASS line, not merely exit 0, is essential: a pattern
// matching zero tests exits 0 with "no tests to run", so a renamed/deleted
// test would otherwise false-GREEN.
```
