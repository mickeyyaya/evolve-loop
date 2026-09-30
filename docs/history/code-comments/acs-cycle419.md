# Comment history: `acs/cycle419`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle419/predicates_test.go:3` — above `package cycle419`

```text
// Package cycle419 materializes the cycle-419 acceptance criteria for the
// committed top_n task:
//
//   - trim-scout-prompt-redundancy (T1) — collapse the run-on §9 eval-
//     materialization paragraph (restates gate #6 evals-materialized) and the
//     multi-cycle war-story in the challenge-token block, without removing any
//     required output section, gate, or frontmatter field.
//
// Deferred (zero predicates per R9.3): trim-auditor-prompt-rationale is NOT in
// the triage top_n (triage-report.md lists only trim-scout-prompt-redundancy).
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	trim-scout-prompt-redundancy (T1):
//	  AC1  agents/evolve-scout.md ≤ 204 lines (was 219)                  → C419_001 (RED)
//	  AC2  byte count strictly < 14941 (baseline 14941)                   → C419_002 (RED)
//	  AC3  frontmatter fields preserved (name/model/description/tools)    → C419_003 (pre-existing GREEN)
//	  AC4  all 11 required output-section names still instructed           → C419_004 (pre-existing GREEN)
//	  AC5  all 6 gates + challenge-token rule present                      → C419_005 (pre-existing GREEN)
//	  AC6  anti-gaming floor ≥150 lines (content not gutted)              → C419_006 (pre-existing GREEN)
//	  AC7  no duplicate ## headings in body                                → C419_007 (pre-existing GREEN)
//	  AC8  go test ./internal/prompts/... ./internal/phases/scout/... green → C419_008 (pre-existing GREEN)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C419_006 — anti-gaming floor (catches over-deletion that games AC1);
//	          "eval materialization" mandate must survive even when §9 prose is compacted.
//	Edge/OOD: C419_002 — boundary: 14941 bytes is the exact baseline; 14941 ≥ 14941 → fails.
//	Semantic:  8 distinct dimensions: line-count / byte-count / frontmatter-fields /
//	           output-section-names / gate-presence / floor-lines / heading-dedup / test-suite.
//
// 1:1 enforcement:
//
//	T1: predicate=8 (C419_001–C419_008), manual+checklist=0, unverifiable-remove=0 → total AC=8 ✓
//	T2 (not top_n): predicate=0 per R9.3 ✓
```
