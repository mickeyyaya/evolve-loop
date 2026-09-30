# Comment history: `acs/cycle426`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle426/predicates_test.go:3` — above `package cycle426`

```text
// Package cycle426 materialises the cycle-426 acceptance criteria for two tasks:
// wiring the driver-bench consumer into applyBenchToPlan (T1) and adding
// ClearBootStrike to restore the documented "consecutive strikes" contract (T2).
//
// Goal: close the two remaining boot-latency leaks — (1) exit-80 boot-strikes
// are recorded per-driver but the consumer (applyBenchToPlan) never routes them
// to ApplyDriverBench, so a driver that repeatedly times out on boot is never
// demoted; (2) strike counts are cumulative not consecutive because there is no
// reset on a successful boot, causing transient-retry drivers to reach the bench
// threshold from non-adjacent failures.
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	wire-driver-bench-consumer (T1 — Medium):
//	  AC1  2-strike codex-tmux demoted behind claude-tmux in dispatch  (positive)   → C426_001 RED
//	  AC2  no boot-bench → no reorder                                  (negative)   → C426_002 pre-GREEN
//	  AC3  all-driver-benched → dispatch not stranded                  (edge)       → C426_003 pre-GREEN
//	  AC4  family bench (rate_limit) still demotes after change        (regression) → C426_004 pre-GREEN
//
//	reset-boot-strike-on-success (T2 — Small):
//	  AC5  ClearBootStrike removes active boot-strike entry             (positive)   → C426_005 RED (compile)
//	  AC6  strike→clear→strike NOT benched                             (negative)   → C426_006 RED (compile)
//	  AC7  ClearBootStrike Reason-scoped; no-entry is no-op            (edge)       → C426_007 RED (compile)
//	  AC8  engine.go calls ClearBootStrike on non-boot exit            (regression) → C426_008 RED (compile)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C426_002 (no bench → no reorder), C426_006 (strike→clear→strike ≠ bench)
//	Edge/OOD:  C426_003 (all-benched not stranded), C426_007 (Reason-scope + no-entry no-op)
//	Semantic:  8 distinct dimensions across reorder/no-op/not-stranded/family-coexist/
//	           remove-entry/consecutive-reset/reason-guard/engine-wire.
//
// 1:1 enforcement:
//
//	T1: predicate=4 (C426_001–004) + manual+checklist=0 → total=4 ✓
//	T2: predicate=4 (C426_005–008) + manual+checklist=0 → total=4 ✓
```
