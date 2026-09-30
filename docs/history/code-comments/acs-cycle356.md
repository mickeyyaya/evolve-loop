# Comment history: `acs/cycle356`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle356/predicates_test.go:3` — above `package cycle356`

```text
// Package cycle356 materializes the cycle-356 acceptance criteria for the
// committed top_n task:
//
//   - budget-cluster-dead-flag-removal — remove 12 dead Budget Cluster flags
//     from registry_table.go, clean production Go references and help text,
//     remove ErrBudgetExceeded dead code, clean skills/docs, regenerate
//     control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	budget-cluster-dead-flag-removal:
//	  AC-1 (neg)  12 Budget Cluster flags absent from flagregistry.Lookup → C356_001
//	  AC-2        TestFlagRegistry_NoBudgetClusterDeadFlags passes          → C356_002
//	  AC-3        EVOLVE_BUILD_PLANNER preserved (not removed)              → C356_003
//	  AC-4        evolve flags check exits 0 (no drift)                     → C356_004
//	  AC-5 (neg)  ErrBudgetExceeded absent from go/internal/core/errors.go  → C356_005
//	  AC-5 (neg)  No EVOLVE_FANOUT_PER_WORKER_BUDGET_USD in cmd help text   → C356_006
//	  AC-6 (neg)  Skills/docs cleaned of budget flag references             → C356_007
//
// Floor binding (R9.3): only committed top_n items get predicates.
// All deferred tasks (deprecated-bridge-retirement, etc.) get zero predicates.
```

### `go/acs/cycle356/predicates_test.go:40` — above `var budgetClusterFlags = []string{`

```text
// budgetClusterFlags is the complete list of the 12 StatusDead Budget Cluster
// flags that cycle-356 removes. Keep in sync with scout-report.md.
```
