# Comment history: `acs/cycle503`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle503/predicates_test.go:3` — above `package cycle503`

```text
// Package cycle503 materialises the cycle-503 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle:
//
//	latest-model-preference: Model-selection policy for ALL CLIs
//	(inbox 2026-07-02T02-00-00Z-latest-model-preference.json, D1-D9,
//	4-tier top-default vocabulary — the sole fleet-scoped commit per
//	triage's rationale; every other backlog item deferred).
//
// SCOPE DECISION (documented, not silent — see test-report.md "Scope
// Decision"): the inbox item is a 9-design-point campaign (D1-D9) far larger
// than one TDD→Builder cycle. Cycle 499 already shipped the first two
// foundational, zero-I/O points (D2 newest-wins comparator + the 4-tier
// vocabulary with the high→deep alias); those are GREEN and out of scope here.
// This cycle advances to the NEXT self-contained, deterministic, zero-I/O
// point that is independently valuable and unblocks the rest:
//
//	D7 (FAMILY CONSTRAINT — family-pure candidate sets) → C503_001..003
//
// D1, D3, D4, D5, D6, D8, D9 and the docs deliverable remain OUT OF SCOPE this
// cycle (they require live-CLI/bridge integration or policy control-plane edits
// this slice does not touch) and stay queued to carryoverTodos — see
// test-report.md.
//
// Predicate strategy (mirrors cycle499/cycle488): behavioral predicates drive
// the new code through its in-package RED tests via subprocess `go test`,
// asserting a non-degenerate pass (requireTestsRan closes the cycle-85
// "no tests matched" trap) — never a source grep. The in-package tests
// (internal/modelquery/family_test.go) were authored by the TDD engineer;
// the Builder implements internal/modelquery/family.go only.
```
