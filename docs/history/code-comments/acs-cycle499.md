# Comment history: `acs/cycle499`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle499/predicates_test.go:3` — above `package cycle499`

```text
// Package cycle499 materialises the cycle-499 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle:
//
//	latest-model-preference: Model-selection policy for ALL CLIs
//	(inbox 2026-07-02T02-00-00Z-latest-model-preference.json, D1-D9,
//	4-tier top-default vocabulary — assigned to THIS concurrent fleet
//	lane per triage's fleet_scope rationale).
//
// SCOPE NOTE (documented, not silent — see test-report.md "Scope Decision"):
// the inbox item is a 9-design-point campaign (D1-D9) spanning live CLI
// query, bridge-probe verification, family filtering, freshness triggers,
// and a conformance suite — far larger than one TDD->Builder cycle. This
// cycle materialises the two foundational, self-contained, zero-I/O design
// points that unblock the rest and are independently valuable:
//
//	D2 (newest-wins version comparator) → C499_001, C499_002
//	TIER VOCABULARY (top tier + high alias) → C499_003, C499_004
//
// D1, D3, D4, D5, D6, D7, D8, D9 and the docs deliverable are OUT OF SCOPE
// this cycle (they require live-CLI/bridge integration this slice does not
// touch) and are queued to carryoverTodos — see test-report.md.
//
// Predicate strategy (mirrors cycle488): behavioral predicates drive the
// new/changed code through in-package RED tests via subprocess `go test`,
// asserting non-degenerate failure (requireTestsRan guard closes the
// cycle-85 "no tests matched" trap) — never a source grep.
```
