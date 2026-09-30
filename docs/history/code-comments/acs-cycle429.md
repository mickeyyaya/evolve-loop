# Comment history: `acs/cycle429`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle429/predicates_test.go:3` — above `package cycle429`

```text
// Package cycle429 materialises the cycle-429 acceptance criteria for two tasks
// comprising slice S1 of the signal-center consolidation campaign.
//
// Goal: Consolidate bridge liveness into ONE unified, concurrency-safe SignalCenter.
// S1 (this cycle): unify the two divergent peak-token extractors into a single
// exported panestream.ExtractResponseTokens, and migrate the stopreview callsite.
//
// Tasks:
//
//	s1-unify-token-extractor (T1 — Small, P0):
//	  Create panestream.ExtractResponseTokens (general: k-form + plain-integer);
//	  delete extractTokenCountLiveness + rxLivenessTokens; migrate ClaudeDetector.Assess.
//
//	s1-migrate-stopreview-callsite (T2 — Small, P1, dependsOn T1):
//	  Route driver_tmux_repl.go + stopreview.go through panestream.ExtractResponseTokens;
//	  delete rxTokens + extractTokenCount; reconcile tokencount_test "↓ 5200 tokens" 0→5200.
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	s1-unify-token-extractor (T1):
//	  AC1  ExtractResponseTokens exported + k-form correct          (positive)  → C429_001 RED (compile fail)
//	  AC2  plain-integer works (superset vs old k-only)             (positive)  → C429_002 RED (compile fail)
//	  AC3  peak-across-matches returns max                          (positive)  → C429_003 RED (compile fail)
//	  AC4  malformed/empty → 0                                     (negative)  → C429_004 RED (compile fail)
//	  AC5  extractTokenCountLiveness absent from panestream         (negative)  → C429_005 RED (symbol present)
//	  AC6  ClaudeDetector token layer still fires (regression)      (positive)  → C429_006 RED (compile fail)
//
//	s1-migrate-stopreview-callsite (T2):
//	  AC7  extractTokenCount absent from bridge pkg                 (negative)  → C429_007 RED (symbol present)
//	  AC8  reconciled case: ↓ 5200 tokens → 5200                   (positive)  → C429_008 RED (compile fail)
//	  AC9  token-usage.json peak invariant preserved                (positive)  → C429_009 RED (compile fail)
//	  AC10 BuildReport.TokenUsage populated from sidecar            (positive)  → C429_010 RED (compile fail)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C429_004 (malformed → 0; no-op returning constant can't pass k-form cases),
//	            C429_005 (old private symbol MUST be absent — fails if still present),
//	            C429_007 (extractTokenCount MUST be absent from bridge — fails if still present)
//	Edge/OOD:   C429_004 (empty pane → 0; missing-arrow → 0; no-digits → 0),
//	            C429_002 (plain-integer 50/200 — OOD for old k-only extractor)
//	Semantic:   C429_001 vs C429_002: k-form and plain-integer are distinct parse paths;
//	            C429_003: peak-max is a reduce behavior, not a single-match behavior
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C429_001–006) → total=6 ✓
//	T2: predicate=4 (C429_007–010) → total=4 ✓
//
// RED strategy:
//
//	C429_001–C429_004, C429_006, C429_008–C429_010:
//	  Shell out to `go test ./internal/bridge/panestream/...` or `./internal/bridge/...`.
//	  liveness_test.go calls undefined ExtractResponseTokens → compile error → exit non-zero → RED.
//	C429_005: FileNotContains — extractTokenCountLiveness IS present in liveness.go → assertion fails → RED.
//	C429_007: FileNotContains — extractTokenCount IS present in stopreview.go → assertion fails → RED.
```

### `go/acs/cycle429/predicates_test.go:135` — above `func TestC429_005_ExtractTokenCountLivenessAbsent(t *testing.T) {`

```text
// TestC429_005_ExtractTokenCountLivenessAbsent (negative, config-check):
// The private extractTokenCountLiveness function and rxLivenessTokens regex
// MUST be absent from panestream/liveness.go after T1 lands. Their presence
// means Builder duplicated instead of unified (the ADR-0047 violation).
// acs-predicate: config-check
```
