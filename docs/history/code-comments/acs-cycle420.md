# Comment history: `acs/cycle420`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle420/predicates_test.go:3` — above `package cycle420`

```text
// Package cycle420 materializes the cycle-420 acceptance criteria for the two
// committed top_n tasks:
//
//   - router-catalog-dedup-overflow (T1) — replace the bare "- also available (<names>)"
//     overflow enumeration in writeCatalog with a one-line pointer to phase-inventory.json,
//     removing ~1KB/cycle of duplicate context from the largest per-cycle prompt.
//
//   - router-persona-tsc-compress (T2) — apply TSC to the prose sections of
//     agents/evolve-router.md (## Your job, ## Output contract, ## Goal-Type Recipes prose),
//     adding the "<!-- TSC applied" marker and achieving ≥15% byte reduction on those
//     sections while keeping the Phase Catalog table byte-identical.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	router-catalog-dedup-overflow (T1):
//	  AC1  writeCatalog overflow → no "- also available (" enumeration   → C420_001 (RED)
//	  AC2  writeCatalog overflow → pointer to phase-inventory.json        → C420_002 (RED)
//	  AC3  negative: even 1-card overflow must have pointer, no enum      → C420_003 (RED)
//	  AC4  edge: no-overflow path unchanged (no pointer, no enum)         → C420_004 (pre-existing GREEN)
//	  AC5  regression: TestRouterCompaction still passes                  → C420_005 (pre-existing GREEN)
//
//	router-persona-tsc-compress (T2):
//	  AC1  TSC marker present in agents/evolve-router.md                  → C420_006 (RED)
//	  AC2  prose region < 5243 bytes (≥15% below 6169-byte baseline)      → C420_007 (RED)
//	  AC3  negative: catalog section byte-identical (7988 bytes)          → C420_008 (pre-existing GREEN)
//	  AC4  edge: domain vocab/code tokens preserved verbatim              → C420_009 (pre-existing GREEN)
//	  AC5  regression: loader + router compaction suite still passes      → C420_010 (pre-existing GREEN)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C420_003 (pointer required even with 1 overflow — catches remove-only gaming);
//	          C420_008 (catalog unchanged — catches TSC gaming by trimming the protected table).
//	Edge/OOD: C420_004 (no overflow → no pointer/enum — boundary at maxEnrichedCatalogCards);
//	          C420_007 (boundary: 5243 bytes exact — 5243 ≥ 5243 → still fails).
//	Semantic:  10 distinct dimensions across 2 tasks: enum-absent / pointer-present /
//	           anti-gaming / no-overflow-clean / compaction-regression / tsc-marker /
//	           prose-bytes / catalog-bytes / vocab-tokens / parse-green.
//
// 1:1 enforcement:
//
//	T1: predicate=5 (C420_001–C420_005), manual+checklist=0, unverifiable-remove=0 → total AC=5 ✓
//	T2: predicate=5 (C420_006–C420_010), manual+checklist=0, unverifiable-remove=0 → total AC=5 ✓
```
