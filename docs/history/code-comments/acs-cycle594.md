# Comment history: `acs/cycle594`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle594/predicates_test.go:3` — above `package cycle594`

```text
// Package cycle594 materialises the cycle-594 acceptance criteria for the one
// triage-committed top_n task (see triage-report.md):
//
//   - memo-phase-tier-envelope (inbox 0.95 critical): (a) the memo phase's
//     model-tier pin must satisfy its profile's model_tier_envelope, config-only;
//     and (b) a PASS-side post-ship observer phase (memo / post-ship-monitor)
//     that fails AFTER a healthy ship must be non-fatal — degraded to a WARN
//     diagnostic, never a cycle-level failure that turns a shipped cycle abnormal.
//
// PRE-EXISTING GREEN (honest RED-phase note): this cycle is a goal-hash lane
// re-run of a task that already shipped to main. Commit 8ff4a85f (same
// goal_hash 805f6ced…) is an ancestor of HEAD; it landed BOTH halves — the
// config alignment (cycle-573: .evolve/profiles/memo.json envelope widened to
// min=fast so the fast pin, rank 1, sits inside [fast..balanced]) and the
// non-fatal post-ship classifier (cycle-574: Orchestrator.postShipObserverSkip,
// wired into cyclerun_dispatch.go). So the criteria below are ALREADY satisfied
// and these predicates run GREEN today rather than RED. They are retained as the
// cycle's audit-gating contract: each exercises the real system under test (not
// a source grep) and would go RED if the shipped config or classifier regressed.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…573 precedent).
// Each predicate shells `go test -run` over the internal-package unit tests that
// materialise the criteria — ValidatePin over the SHIPPED config, and the real
// Orchestrator.postShipObserverSkip classifier over a real catalog/cfg. None is
// a source-grep.
```
