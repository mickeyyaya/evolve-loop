# Comment history: `acs/cycle504`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle504/predicates_test.go:3` — above `package cycle504`

```text
// Package cycle504 materialises the cycle-504 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle:
//
//	latest-model-preference: model-selection policy for all CLIs (conformance)
//	(inbox 2026-07-02T02-00-00Z-latest-model-preference.json, D1-D9, 4-tier
//	top-default vocabulary — the sole fleet-scoped commit per triage's
//	rationale; every other backlog/inbox item, including this cycle's own
//	scout-report.md fleet-width tasks, deferred).
//
// SCOPE DECISION (documented, not silent — see test-report.md "Scope
// Decision"): scout-report.md for cycle 504 researched a DIFFERENT inbox item
// (triage-supply-disjoint-topn-for-fleet-width); triage instead committed
// latest-model-preference to top_n (assigned fleet scope), so this cycle's
// tests are authored directly against the inbox item + triage-report.md
// rationale, not scout-report.md. The inbox item is a 9-design-point campaign
// (D1-D9) far larger than one TDD->Builder cycle. Prior cycles already shipped
// two foundational, zero-I/O slices: cycle499 (D2 newest-wins comparator + the
// 4-tier vocabulary with the high->deep alias) and cycle503 (D7's standalone
// FamilyOf/FilterByFamily helpers, built but NOT wired into production). This
// cycle advances to the natural next increment: WIRE D7 into production —
//
//	D7 PRODUCTION WIRING (policy.json catalog.allowed_families -> the live
//	refresh pipeline, so a CLI's candidate ids are family-filtered BEFORE they
//	reach the Classifier) -> C504_001..003
//
// D1, D2(wiring), D3, D4, D5, D6, D8, D9 and the docs deliverable remain OUT OF
// SCOPE this cycle (they require live-CLI/bridge integration this slice does
// not touch) and stay queued to carryoverTodos — see test-report.md.
//
// Predicate strategy (mirrors cycle499/cycle503): behavioral predicates drive
// the new code through its in-package RED tests via subprocess `go test`,
// asserting a non-degenerate pass (requireTestsRan closes the cycle-85 "no
// tests matched" trap) — never a source grep. The in-package tests
// (internal/policy/policy_test.go, internal/modelquery/query_test.go) were
// authored by the TDD engineer; the Builder implements
// internal/policy/policy.go + internal/modelquery/query.go only.
```

### `go/acs/cycle504/predicates_test.go:99` — above `func TestC504_002_RefreshFiltersByFamilyBeforeClassify(t *testing.T) {`

```text
// TestC504_002_RefreshFiltersByFamilyBeforeClassify (D7-AC2, the live-evidence
// acceptance scenario): RefreshDeps.AllowedFamilies must filter a CLI's
// live-queried id list down to its allowed families BEFORE the ids reach the
// Classifier — reproducing the exact agy incident (a mixed Claude/Gemini/GPT
// list must never let a cross-family id reach classification, the root cause
// of the observed Sonnet-4.6->GPT-OSS-120B flap). Drives modelquery.Refresh
// through its in-package test. RED today: RefreshDeps has no AllowedFamilies
// field (modelquery package build failure).
```
