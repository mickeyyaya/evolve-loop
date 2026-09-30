# Comment history: `acs/cycle424`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle424/predicates_test.go:3` — above `package cycle424`

```text
// Package cycle424 materializes the cycle-424 acceptance criteria for two tasks:
// a driver-scoped boot-timeout bench mechanism (T1) and an ollama liveness strategy
// composed over DefaultDetector (T2).
//
// Goal: reduce cycle latency by eliminating repeated dead-boot cycles (T1) and
// demonstrating the liveness abstraction is genuinely extensible (T2).
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	repl-boot-timeout-driver-bench (T1 — Medium):
//	  AC1  BootTimeoutPattern constant and Benchable("repl_boot_timeout")=true  → C424_001
//	  AC2  IsBootTimeoutExitCode(80)=true; IsBootTimeoutExitCode(81)=false       → C424_002
//	  AC3  ≥threshold consecutive RecordBootStrike benches the driver (positive)  → C424_003
//	  AC4  Single RecordBootStrike does NOT bench (negative — threshold guard)    → C424_004
//	  AC5  Driver-scoped: "codex-tmux" bench does NOT bench "codex" (anti-gaming) → C424_005
//
//	ollama-liveness-strategy (T2 — Small):
//	  AC6  OllamaDetector → Converging+higher-conf on "Thinking…" frame w/ no content delta → C424_006
//	  AC7  No "Thinking…" header → OllamaDetector byte-identical to DefaultDetector         → C424_007
//	  AC8  Malformed/empty frames → no panic (edge/OOD)                                     → C424_008
//	  AC9  DetectorFor("ollama") routes to OllamaDetector (conf uplift vs Default)          → C424_009
//	  AC10 stopreview.go has no CLI-name literals (config-check waiver; pre-existing GREEN)  → C424_010
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C424_004 (1 strike < threshold → no bench), C424_007 (no-Thinking fallback)
//	Edge/OOD:  C424_002 (non-80 not a boot exit), C424_005 (driver-scoped isolation), C424_008 (no panic)
//	Semantic:  10 distinct dimensions: pattern-exists / exit-code-mapping /
//	           threshold-crossed / single-strike / driver-scope-isolation /
//	           thinking-converging / thinking-absent-fallback / edge-no-panic /
//	           detector-routing / no-cli-branch.
//
// 1:1 enforcement:
//
//	T1: predicate=5 (C424_001–C424_005), manual=0, unverifiable=0 → total AC=5 ✓
//	T2: predicate=5 (C424_006–C424_010), manual=0, unverifiable=0 → total AC=5 ✓
```

### `go/acs/cycle424/predicates_test.go:347` — above `func TestC424_010_NoCliNameInStopReview(t *testing.T) {`

```text
// TestC424_010_NoCliNameInStopReview asserts stopreview.go still contains no
// hardcoded CLI-name string literals after T1 and T2 changes. Pre-existing GREEN:
// the reviewer was already branch-free (cycle-423 C423_007). This predicate is the
// regression guard — adding an ollama DetectorFor branch in the reviewer violates
// the abstraction. Declared // acs-predicate: config-check per the single-source waiver
// (the only mechanical way to enforce the zero-CLI-branch design constraint).
```
