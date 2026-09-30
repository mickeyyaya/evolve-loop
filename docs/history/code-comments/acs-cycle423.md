# Comment history: `acs/cycle423`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle423/predicates_test.go:3` — above `package cycle423`

```text
// Package cycle423 materializes the cycle-423 acceptance criteria for three
// liveness-detection tasks: a DefaultDetector strategy (T1), reviewer integration
// that consumes LivenessState instead of raw booleans (T2), and a claude-specific
// token-counter layer composed over the default (T3).
//
// Goal: reduce cycle latency by making phase-agent liveness detection more
// accurate, via a clean abstract Strategy layer — driver-agnostic, zero per-CLI
// branching in the consumer (stopreview.go).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	liveness-detector-default-strategy (T1 — Medium):
//	  AC1  LivenessState enum {Converging|BusyButStagnant|Idle|Hung} exists      → C423_001 (RED)
//	  AC2  DefaultDetector → Converging for all 4 CLIs (thinking→answer)         → C423_002 (RED)
//	  AC3  DefaultDetector → BusyButStagnant for busy-but-no-content (negative)  → C423_003 (RED)
//	  AC4  DefaultDetector → Idle for quiet frame (edge: not Hung immediately)   → C423_004 (RED)
//	  AC5  DefaultDetector → Hung after stallThreshold consecutive stall rounds  → C423_005 (RED)
//	  AC6  DetectorFor registry returns LivenessProbe for all 4 CLIs             → C423_006 (RED)
//
//	liveness-detector-reviewer-integration (T2 — Medium):
//	  AC7  No CLI-name literal in stopreview.go (no-branch invariant)            → C423_007 (pre-existing GREEN)
//	  AC8  StopEvent.State field of type panestream.LivenessState exists          → C423_008 (RED)
//	  AC9  Converging → ReviewExtend past maxExtends (unconditional)             → C423_009 (RED)
//	  AC10 Hung → ReviewPause fast-fail under maxExtends                         → C423_010 (RED)
//
//	liveness-claude-tokencount-strategy (T3 — Small):
//	  AC11 Increasing ↓ token series → Converging with conf > DefaultDetector    → C423_011 (RED)
//	  AC12 Static token counter → same state as DefaultDetector (negative)       → C423_012 (RED)
//	  AC13 Malformed token line → no panic, default verdict (edge)               → C423_013 (RED)
//	  AC14 Non-claude CLIs via DetectorFor → same state as DefaultDetector       → C423_014 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C423_003 (busy-but-no-content ≠ Converging), C423_012 (static token = default)
//	Edge/OOD: C423_004 (quiet ≠ Hung on single interval), C423_013 (malformed = no panic)
//	Semantic:  14 distinct dimensions: enum / 4CLIs-Converging / BusyButStagnant /
//	           Idle / Hung / registry / no-CLI-branch / State-field /
//	           Converging-uncapped / Hung-fastfail / increasing-token /
//	           static-fallback / malformed-nopanic / non-claude-unaffected.
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C423_001–C423_006), manual=0, unverifiable=0 → total AC=6 ✓
//	T2: predicate=4 (C423_007–C423_010), manual=0, unverifiable=0 → total AC=4 ✓
//	T3: predicate=4 (C423_011–C423_014), manual=0, unverifiable=0 → total AC=4 ✓
```

### `go/acs/cycle423/predicates_test.go:259` — above `func TestC423_009_ConvergingExtendsUnconditionally(t *testing.T) {`

```text
// TestC423_009_ConvergingExtendsUnconditionally asserts the reviewer returns ReviewExtend
// for State=Converging even at an attempt number past maxExtends. A converging agent
// must NEVER be capped — this is the fix for cycles 311/312 where a producing scout
// was killed mid-work by the backstop.
// RED: bridge.NewDeterministicReviewer or StopEvent.State absent → compile error.
```
