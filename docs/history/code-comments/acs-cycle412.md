# Comment history: `acs/cycle412`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle412/predicates_test.go:3` — above `package cycle412`

```text
// Package cycle412 materializes the cycle-412 acceptance criteria for two prompt-optimization tasks:
//   - prune-dead-legacy-script-refs (agents/evolve-{scout,builder,auditor,tdd-engineer,orchestrator,triage}.md)
//   - dedupe-phase-prompt-reference-index (agents/evolve-{scout,builder,auditor}.md)
//
// Goal: optimize per-agent token usage by removing 18 dead legacy/scripts refs, 5 v12.0.0
// disclaimer blocks, and collapsing 30 repeated reference-file paths (9/12/9) to ≤2 each.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	prune-dead-legacy-script-refs:
//	  AC1 zero legacy/scripts refs across 6 prompt files (18 currently)  → C412_001 (RED)
//	  AC2 combined word count < 14126 (currently exactly 14126)           → C412_002 (RED)
//	  AC3 gate anchors preserved (STOP CRITERION, challenge-token, etc)   → C412_003 (pre-existing GREEN, config-check)
//	  AC4 no file gutted (each prompt ≥ 100 lines)                        → C412_004 (pre-existing GREEN)
//	  AC5 v12.0.0 status disclaimers removed (5 currently)                → C412_005 (RED)
//
//	dedupe-phase-prompt-reference-index:
//	  AC1 reference path repeated ≤ 2× per file (scout 9, builder 12, auditor 9 currently) → C412_006 (RED)
//	  AC2 all 7 scout reference section names present after collapse                         → C412_007 (pre-existing GREEN, config-check)
//	  AC3 ≥1 pointer per file (not amputated wholesale)                                      → C412_008 (pre-existing GREEN)
//	  AC4 combined scout+builder+auditor word count < 6861 (currently exactly 6861)          → C412_009 (RED)
//
// Adversarial diversity (per SKILL §6):
//
//	Negative: legacy/scripts present → C412_001 (RED); v12.0.0 disclaimer present → C412_005 (RED);
//	          path repeat > 2 → C412_006 (RED).
//	Edge/OOD: file gutted (< 100 lines) → C412_004; pointer amputated entirely → C412_008.
//	Semantic:  word-count reduction (distinct from string-absence) → C412_002, C412_009.
//
// Deferred (zero predicates per R9.3): 27 carryover breadcrumbs (all infra/codex-tmux boot-wedge class).
```

### `go/acs/cycle412/predicates_test.go:110` — above `func TestC412_002_CombinedWordCountReduced(t *testing.T) {`

```text
// TestC412_002_CombinedWordCountReduced asserts that the total word count
// across all six phase-prompt files is strictly less than 13900.
//
// BEHAVIORAL: reads and counts whitespace-delimited tokens (strings.Fields semantics).
// Removing 20 dead legacy/scripts instruction occurrences plus 5 v12.0.0 disclaimer
// blocks reduces the word count by ~300–500 words from the Go-measured baseline of
// 14124. The threshold 13900 sits between the baseline and the expected post-cleanup
// floor; Builder must perform the actual dead-ref removal to pass it.
//
// NEGATIVE: an unmodified tree totals 14124 words (> 13900), so this fails.
// Only genuine removal of dead instructions satisfies it.
//
// RED: combined Go-measured baseline = 14124 (scout 1915, builder 2683, auditor 2263,
// tdd 2801, orchestrator 2356, triage 2106). The scout report cited 14126 via wc -w;
// Go's strings.Fields measures 14124 for the same files.
```

### `go/acs/cycle412/predicates_test.go:243` — above `func TestC412_005_NoV12StatusDisclaimers(t *testing.T) {`

```text
// TestC412_005_NoV12StatusDisclaimers asserts that all five
// "> **v12.0.0 status:**" disclaimer blocks have been removed from the six
// phase-prompt files.
//
// BEHAVIORAL: counts "v12.0.0 status" occurrences per file. The disclaimers
// exist solely to neutralize the dead legacy/scripts references — once those
// references are removed (AC1), the disclaimers are pure workaround weight
// and must be deleted too (CLAUDE rule no_workaround_root_cause_redesign).
//
// NEGATIVE (adversarial): the unmodified tree has 5 disclaimers (scout 1,
// builder 1, auditor 1, tdd 1, triage 1; orchestrator 0) — all must go.
//
// RED: currently 5 occurrences total.
```
