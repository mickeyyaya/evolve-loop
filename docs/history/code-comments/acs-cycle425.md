# Comment history: `acs/cycle425`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle425/predicates_test.go:3` — above `package cycle425`

```text
// Package cycle425 materializes the cycle-425 acceptance criteria for two tasks:
// production wiring of the boot-timeout bench store into adapters.NewDefault (T1)
// and the agy liveness spinner strategy composed over DefaultDetector (T2).
//
// Goal: close the dead latency levers — the BootTimeoutStore wiring gap (no strikes
// ever recorded in production; engine.go:455 is a no-op with nil store) and the
// missing per-CLI strategy for agy (falls through to bare DefaultDetector, missing
// the ⣯ Generating... / esc to cancel affordance as a Converging signal).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	wire-boottimeout-store-production (T1 — Medium):
//	  AC1  adapters.NewDefault sets BootTimeoutStore non-nil (positive)            → C425_001
//	  AC2  Single RecordBootStrike does NOT bench (negative/threshold guard)       → C425_002
//	  AC3  exit-81 (artifact timeout) is NOT classified as boot timeout (edge)     → C425_003
//	  AC4  engine+clihealth+llmroute suites green (regression)                    → manual+checklist
//
//	agy-liveness-spinner-strategy (T2 — Small):
//	  AC5  AgyDetector generating frame ⇒ LivenessConverging conf≥0.9 (positive)  → C425_004
//	  AC6  AgyDetector answer frame ⇒ DefaultDetector byte-identical (negative)   → C425_005
//	  AC7  DetectorFor("agy") → *AgyDetector + conf-uplift on generating frame    → C425_006
//	  AC8  stopreview.go has no CLI-name literals after T1+T2 (regression)        → C425_007
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C425_002 (1 strike < threshold → not benched), C425_005 (answer frame → default fallback)
//	Edge/OOD:  C425_003 (exit-81 ≠ boot timeout), C425_006 (type assertion + registry routing)
//	Semantic:  7 distinct dimensions: production-wiring / threshold-guard / exit-code-map /
//	           spinner-converging / answer-fallback / registry-route / no-cli-literal.
//
// 1:1 enforcement:
//
//	T1: predicate=3 (C425_001–C425_003) + manual+checklist=1 (AC4) → total=4 ✓
//	T2: predicate=4 (C425_004–C425_007) + manual=0                 → total=4 ✓
```

### `go/acs/cycle425/predicates_test.go:78` — above `func TestC425_002_SingleStrikeDoesNotBench(t *testing.T) {`

```text
// TestC425_002_SingleStrikeDoesNotBench is the load-bearing negative test:
// a single RecordBootStrike call must NOT bench the driver — one transient boot
// failure must remain retryable. Only consecutive failures at the threshold bench.
// Pre-existing GREEN (clihealth mechanism landed cycle 424); kept as regression guard
// for this cycle's production wiring.
```

### `go/acs/cycle425/predicates_test.go:101` — above `func TestC425_003_WrongExitCodeNotBootTimeout(t *testing.T) {`

```text
// TestC425_003_WrongExitCodeNotBootTimeout asserts that exit code 81
// (ExitREPLArtifactTimeout) is NOT classified as a boot timeout. A misclassification
// here would cause artifact-timeout dispatches to accumulate spurious boot strikes
// and bench a healthy driver after 2 artifact timeouts.
// Pre-existing GREEN (IsBootTimeoutExitCode correct since cycle 424); kept as regression guard.
```

### `go/acs/cycle425/predicates_test.go:219` — above `func TestC425_007_NoCliNameInStopReview(t *testing.T) {`

```text
// TestC425_007_NoCliNameInStopReview asserts stopreview.go still contains no
// hardcoded CLI-name string literals after T1 and T2 changes. The reviewer must
// remain branch-free — zero per-CLI logic — as pinned by C424_010 and the
// ADR-0047 projection seam. Adding AgyDetector to DetectorFor must NOT tempt an
// agy branch into the reviewer.
// Pre-existing GREEN (C424_010 guard); this predicate is the regression continuation.
// Declared // acs-predicate: config-check per the single-source waiver.
```
