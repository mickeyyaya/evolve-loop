# Comment history: `acs/cycle421`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle421/predicates_test.go:3` — above `package cycle421`

```text
// Package cycle421 materializes the cycle-421 acceptance criteria for two prompt-compaction tasks:
//
//   - retro-phase-compaction-wiring (T1) — wire CompactPrompts into retro phase (only content
//     phase that loads an agent doc outside the BaseRunner compaction path), and rebalance
//     evolve-retrospective.md so ≥1500B is stripped per retro invocation.
//
//   - orchestrator-reference-index-rebalance (T2) — relocate on-demand sections of
//     evolve-orchestrator.md below ## Reference Index so ≥2000B is stripped; add
//     TestOrchestratorCompaction byte-floor + anchor-survival test mirroring cycle 415-417 pattern.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	retro-phase-compaction-wiring (T1):
//	  AC1  evolve-retrospective.md saves ≥1500B after StripOnDemandSections → C421_001 (RED)
//	  AC2  retro output-contract anchors survive strip (above marker)          → C421_002 (pre-existing GREEN)
//	  AC3  versioned sections absent from stripped body (negative)             → C421_003 (RED)
//	  AC4  retro.Config has CompactPrompts bool field                          → C421_004 (RED)
//	  AC5  CompactPrompts=true → bridge receives stripped prompt               → C421_005 (RED)
//	  AC6  CompactPrompts=false → prompt byte-identical to raw body            → C421_006 (pre-existing GREEN)
//
//	orchestrator-reference-index-rebalance (T2):
//	  AC1  evolve-orchestrator.md saves ≥2000B after StripOnDemandSections     → C421_007 (RED)
//	  AC2  stripped head ≥9000B (anti-over-strip guard)                        → C421_008 (pre-existing GREEN)
//	  AC3  gate-bearing anchors survive strip (above marker)                   → C421_009 (pre-existing GREEN)
//	  AC4  on-demand sections absent from stripped body (negative)             → C421_010 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: C421_003 (versioned retro sections still above marker → fail);
//	          C421_010 (path/worktree/closure sections still above marker → fail).
//	Edge/OOD: C421_008 (boundary: head must be ≥9000B even if aggressive relocation);
//	          C421_006 (disabled-path must be byte-identical — catch accidental always-strip).
//	Semantic:  10 distinct dimensions: byte-delta retro / retro-anchor-survival /
//	           retro-versioned-absent / config-field / prompt-stripped / identity /
//	           byte-delta orchestrator / head-floor / gate-anchor / on-demand-absent.
//
// 1:1 enforcement:
//
//	T1: predicate=6 (C421_001–C421_006), manual+checklist=0, unverifiable-remove=0 → total AC=6 ✓
//	T2: predicate=4 (C421_007–C421_010), manual+checklist=0, unverifiable-remove=0 → total AC=4 ✓
```
