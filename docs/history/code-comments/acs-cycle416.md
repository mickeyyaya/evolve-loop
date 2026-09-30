# Comment history: `acs/cycle416`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle416/predicates_test.go:3` — above `package cycle416`

```text
// Package cycle416 materializes the cycle-416 acceptance criteria for two prompt-compaction tasks:
//   - intent-ondemand-reference-tail (T1)
//   - prompt-compaction-coverage-gate (T2)
//
// Goal: close the dead-wired compaction in evolve-intent.md (marker absent → 0 bytes stripped
// per cycle despite CompactPrompts=true), and add a regression guard asserting all 7 per-cycle
// agents are strictly compacted.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	intent-ondemand-reference-tail (T1):
//	  AC1 evolve-intent.md has a line-anchored ## Reference Index heading              → C416_001 (RED)
//	  AC2 StripOnDemandSections saves ≥500 bytes from evolve-intent.md body            → C416_002 (RED)
//	  AC3 required operational anchors survive above the ## Reference Index marker      → C416_003 (pre-existing GREEN)
//	  AC4 reference-grade sections (## Composition) absent from stripped body (neg)    → C416_004 (RED)
//
//	prompt-compaction-coverage-gate (T2):
//	  AC1 go/internal/prompts/compaction_coverage_test.go exists                       → C416_005 (RED)
//	  AC2 compaction_coverage_test.go contains "evolve-intent"                         → C416_006 (RED)
//	  AC3 compaction_coverage_test.go names all 7 per-cycle agents                     → C416_007 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: ## Composition in stripped body → C416_004 (stripped==body when no marker → section present → RED).
//	          synthetic markerless body unchanged → C416_NEG (pre-existing GREEN, anti-gaming sentinel).
//	Edge/OOD: C416_003 — inline mention does not trigger strip (mirrors C415_008 pre-existing GREEN).
//	Semantic: marker-presence (C416_001) vs byte-delta (C416_002) vs anchor-survival (C416_003) vs
//	          reference-absent (C416_004) vs file-exists (C416_005) vs content-coverage (C416_006/007)
//	          = 7 distinct dimensions.
//
// Deferred (zero predicates per R9.3): supporting-agent audit (evolve-failure-advisor.md,
// evolve-reflector.md — beyond-ask B1), build-time marker lint (beyond-ask B2).
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	T1: predicate=4 (C416_001–C416_004), manual+checklist=0, unverifiable-remove=0 → total AC=4 ✓
//	T2: predicate=3 (C416_005–C416_007), manual+checklist=0, unverifiable-remove=0 → total AC=3 ✓
//	Adversarial: C416_NEG (synthetic negative, pre-existing GREEN) → total=1 sentinel
```

### `go/acs/cycle416/predicates_test.go:171` — above `func TestC416_005_CompactionCoverageTestExists(t *testing.T) {`

```text
// TestC416_005_CompactionCoverageTestExists asserts that the coverage gate test file
// go/internal/prompts/compaction_coverage_test.go exists.
// This is the regression guard against silent per-cycle token re-inflation when a
// future agent is added without a ## Reference Index marker.
//
// RED: file does not exist (written by TDD engineer in cycle-416, not yet present).
```
