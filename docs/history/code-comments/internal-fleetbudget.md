# Comment history: `internal/fleetbudget`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d3

### `go/internal/fleetbudget/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — ADR-0050 Phase 5 public-API coverage: name and
// exercise every exported fleetbudget symbol by identifier (apicover counts
// field access as "uses", not "names"). Each test asserts a REAL contract.
```

### `go/internal/fleetbudget/fleetbudget.go:1` — above `package fleetbudget`

```text
// Package fleetbudget is the pure allocator of the quota-driven budgeting
// design: given each CLI family's measured quota (quotastate.QuotaState), the
// pipeline's measured pace (budgethistory.Throughput), the operator lane
// envelope + budget tunables (Config), and the current instant, it decides how
// many fleet lanes to run this wave and how to pace them — with a legible trail
// of WHY (DerivedFrom + Reason).
//
// It replaces the static min_lanes assertion (#303) with MEASUREMENT: size the
// wave against real remaining headroom over the time-to-reset, in each CLI's
// NATIVE units (remaining fraction + reset time), NEVER dollars — the reason the
// old dollar budget was removed (subscription CLIs report $0). min_lanes is
// demoted to Config.Floor: the honest lower bound the plan never drops below,
// and the all-unknown fallback.
//
// Degrade-gracefully + shadow-safe: the budget branch activates ONLY when the
// operator has supplied both budget tunables (CapacityCycles>0, 0<Safety≤1).
// Absent them — the default until an operator opts in via the fleet.budget
// policy block — Plan never sizes below Config.Count, so wiring it in shadow is
// byte-identical to today. Pure: no I/O, injectable now. Leaf: depends only on
// the two measurement packages (quotastate, budgethistory) + stdlib; the wiring
// layer maps policy.FleetConfig + the fleet.budget block into Config, keeping
// this allocator free of the policy package.
```
