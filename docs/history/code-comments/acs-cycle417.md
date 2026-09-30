# Comment history: `acs/cycle417`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle417/predicates_test.go:3` — above `package cycle417`

```text
// Package cycle417 materializes the cycle-417 acceptance criteria for two prompt-compaction tasks:
//   - router-catalog-prose-compaction (T1)
//   - reflector-reference-ondemand-split (T2)
//
// Goal: reduce per-agent token usage by (T1) compacting the verbose 66-row router catalog
// prose in-place and (T2) externalizing the evolve-reflector.md historical narrative tail
// via the proven on-demand-marker pattern.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	router-catalog-prose-compaction (T1):
//	  AC1 "## Phase Catalog — Core Values" section < 8000B (was ~10507B)              → C417_001 (RED)
//	  AC2 all 66 phase rows retained (no row deleted by prose trim)                    → C417_002 (pre-existing GREEN)
//	  AC3 no empty-trigger rows in catalog after compaction (anti-gaming)              → C417_003 (pre-existing GREEN)
//
//	reflector-reference-ondemand-split (T2):
//	  AC1 evolve-reflector.md has line-anchored ## Reference Index heading             → C417_004 (RED)
//	  AC2 StripOnDemandSections saves ≥200 bytes from evolve-reflector.md body        → C417_005 (RED)
//	  AC3 required operational anchors survive above the ## Reference Index marker     → C417_006 (pre-existing GREEN)
//	  AC4 "## Why this agent exists" absent from stripped body (negative)              → C417_007 (RED)
//	  AC5 evolve-reflector-reference.md exists and is non-empty                       → C417_008 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: empty trigger in catalog row → C417_003 (catches over-trim); "## Why this
//	          agent exists" in stripped body → C417_007 (catches missed marker placement);
//	          synthetic buried content unchanged → C417_NEG (pre-existing GREEN).
//	Edge/OOD: C417_002 — row count exactly 66, not ≥66 (catches deletion-by-compaction);
//	          C417_003 — trailing pipe stripped from trigger column.
//	Semantic: byte-delta (C417_001/C417_005) vs row-count (C417_002) vs trigger-content (C417_003)
//	          vs marker-presence (C417_004) vs anchor-survival (C417_006) vs section-absent (C417_007)
//	          vs stub-exists (C417_008) = 8 distinct dimensions.
//
// Deferred (zero predicates per R9.3): router catalog projection from phase-metadata SSOT
// (beyond-ask B1), per-agent prompt byte-budget ratchet (beyond-ask B2),
// envelope injection trim (beyond-ask B3).
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	T1: predicate=3 (C417_001–C417_003), manual+checklist=0, unverifiable-remove=0 → total AC=3 ✓
//	T2: predicate=5 (C417_004–C417_008), manual+checklist=0, unverifiable-remove=0 → total AC=5 ✓
//	Adversarial: C417_NEG (synthetic negative, pre-existing GREEN) → total=1 sentinel
```
