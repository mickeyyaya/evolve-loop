# Comment history: `acs/cycle427`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle427/adversarial_probe_test.go:5` — above `import (`

```text
// Amplification probes — characterize behavior of the cycle-427 implementation
// for edge cases not covered by AC1–AC16. These run under the `acs` build tag
// alongside the canonical 16 predicates.
```

### `go/acs/cycle427/predicates_test.go:3` — above `package cycle427`

```text
// Package cycle427 materialises the cycle-427 acceptance criteria for three tasks:
// T1 (parallel-evaluate-policy-injection), T2 (bridge-driver-liveness-routing-test),
// T3 (codex-stalled-liveness-regression).
//
// Goal: close residual gaps left after cycles 423–426 built and wired the
// LivenessDetector abstraction:
//   - T1: ParallelEvaluate lever blocked by a config-not-code violation
//     (config.go:248 has a hardcoded StageOff literal, no policy block exists).
//   - T2: bridge-level detectorFor seam has no test pinning it against regression.
//   - T3: codex stalled→Idle behavior (weak-signal closure) is undocumented by test.
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	parallel-evaluate-policy-injection (T1 — Medium, P0):
//	  AC1  absent block → default Stage="off", Concurrency=3          (positive)  → C427_001 RED (policy compile fail)
//	  AC2  stage "shadow" override persists                            (positive)  → C427_002 RED (policy compile fail)
//	  AC3  stage "enforce" override persists                           (positive)  → C427_003 RED (policy compile fail)
//	  AC4  unknown stage falls back to "off" (fail-safe, anti-enable) (negative)  → C427_004 RED (policy compile fail)
//	  AC5  zero/negative concurrency defaults to 3                    (edge)      → C427_005 RED (policy compile fail)
//	  AC6  defaults().RolloutStages.ParallelEvaluate == StageOff      (safety)    → C427_006 pre-GREEN
//
//	bridge-driver-liveness-routing-test (T2 — Small, P1):
//	  AC7  claude-tmux → ClaudeDetector                               (positive)  → C427_007 pre-GREEN
//	  AC8  codex-tmux → DefaultDetector                               (positive)  → C427_008 pre-GREEN
//	  AC9  agy-tmux → AgyDetector                                     (positive)  → C427_009 pre-GREEN
//	  AC10 ollama-tmux → OllamaDetector                               (positive)  → C427_010 pre-GREEN
//	  AC11 unknown-tmux → DefaultDetector, never nil                  (negative)  → C427_011 pre-GREEN
//	  AC12 stopreview.go has zero CLI literals                        (grep)      → C427_012 pre-GREEN
//
//	codex-stalled-liveness-regression (T3 — Small, P2):
//	  AC13 thinking→answer growth → LivenessConverging                (positive)  → C427_013 pre-GREEN
//	  AC14 prime call never LivenessHung                              (edge)      → C427_014 pre-GREEN
//	  AC15 stalled ≥3 intervals → LivenessIdle, never LivenessHung   (negative)  → C427_015 pre-GREEN
//	  AC16 confidence ∈ [0,1] across all frames                      (semantic)  → C427_016 pre-GREEN
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C427_004 (unknown stage must NOT enable), C427_011 (unknown driver never nil/coarse),
//	           C427_015 (stalled codex must NEVER produce Hung)
//	Edge/OOD:  C427_005 (zero/negative concurrency), C427_014 (priming guard)
//	Semantic:  C427_006 (safety-pin: StageOff default, blind-flip guard),
//	           C427_016 (confidence range invariant)
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C427_001–006)  → total=6 ✓
//	T2: predicate=6 (C427_007–012) → total=6 ✓
//	T3: predicate=4 (C427_013–016) → total=4 ✓
//
// RED strategy:
//
//	T1 predicates invoke `go test ./internal/policy/ -run TestParallelEvaluateConfig_*`
//	via subprocess. The policy test file (parallel_evaluate_config_test.go) references
//	policy.ParallelEvaluatePolicy which doesn't exist → compile error in subprocess →
//	predicate fails. This keeps the ACS package itself compile-clean so T2/T3 predicates
//	run independently.
//	T2 and T3 predicates are pre-existing GREEN (production code already correct).
```

### `go/acs/cycle427/predicates_test.go:217` — above `func TestC427_012_StopReviewHasNoCLILiterals(t *testing.T) {`

```text
// TestC427_012_StopReviewHasNoCLILiterals (grep, pre-GREEN):
// stopreview.go must contain zero CLI-name literals. All per-CLI branching must
// live in panestream.DetectorFor (ADR-0047 single-source-with-projection).
```
