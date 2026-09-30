# Comment history: `acs/cycle1724`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1724/predicates_test.go:3` — above `package cycle1724`

```text
// Package cycle1724 materialises the cycle-1724 acceptance criteria for this
// lane's single fleet-scoped task: remove-dormant-inboxbatch-dependency-grouping.
//
// Scope (per scout-report.md / triage-report.md, item
// inboxbatch-dormant-dependency-grouping): under ADR-0106 W3 dependency_blocked
// routing, a dependent is never offered in the same lane menu as its unlanded
// dependency, so `depRule` (go/internal/inboxbatch/rules.go) and the
// deps-ordering half of `topoOrder` (go/internal/inboxbatch/classify.go) are
// live-but-unreachable in production dispatch — dead code with passing tests.
// campaign grouping and the opt-in `ConnectsRule` are unaffected and must keep
// working exactly as today.
//
// Predicate strategy — every predicate below EXERCISES the system under test
// (calls inboxbatch.DefaultRules()/Classify, or renders the real triage prompt
// through triage.New(...).ComposePrompt); none is a source-grep of production
// text (the cycle-85 degenerate-predicate ban).
//
//   - 001 (AC1a) DefaultRules() must be exactly the two remaining structural
//     rules (campaign, file-area) — no depRule — AND items whose only shared
//     signal is a Deps chain must bind zero edges.
//   - 002 (AC1b) Classify's ordering must no longer respect Deps: a cluster
//     bound by campaign, with a heavier child declaring a dependency on a
//     lighter parent, must order weight-desc (child first), not topological
//     (parent first).
//   - 003 (AC3, regression guard) campaign grouping and the opt-in ConnectsRule
//     stay byte-for-byte unaffected — pre-existing GREEN today, must remain
//     GREEN after the deps removal.
//   - 004 (AC2) the real triage prompt's `inbox_batches` wording must name
//     exactly the grouping signals the triage path applies. It renders the
//     prompt over an inbox whose pairs share one signal each and checks the
//     wording against which pairs were really co-batched: campaign and
//     file-area must be named, connects_to (or "links") may be named only if
//     the connects_to pair is co-batched, and no dependency language is allowed
//     because a dependent is routed to dependency_blocked instead.
//   - 005 (AC5) the checked-in historical ACS predicates that pinned the
//     three-rule DefaultRules() (cycle1205, cycle1206, cycle1633) are
//     re-derived to the two-rule set and pass. The named tests must still run
//     and PASS, so deleting them does not count as a fix.
```
