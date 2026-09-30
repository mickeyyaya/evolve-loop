# Comment history: `acs/cycle411`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle411/predicates_test.go:3` — above `package cycle411`

```text
// Package cycle411 materializes the cycle-411 acceptance criteria for three TSC tasks:
//   - tsc-compress-auditor-prompt (agents/evolve-auditor.md, baseline 22137 bytes)
//   - tsc-compress-tdd-engineer-prompt (agents/evolve-tdd-engineer.md, baseline 25544 bytes)
//   - tsc-compress-orchestrator-prompt (agents/evolve-orchestrator.md, baseline 24642 bytes)
//
// Telegraphic Semantic Compression (TSC) removes grammar glue while preserving every
// section header, code-span line, and gate anchor.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	tsc-compress-auditor-prompt:
//	  AC1 bytes < 18816 (≥15% cut from 22137)                    → C411_001 (RED)
//	  AC2 TSC marker present (<!-- TSC applied)                   → C411_002 (RED)
//	  AC3 header count == 25 exact (anti-gaming: no deletion)     → C411_003 (pre-existing GREEN, config-check)
//	  AC4 code-span lines ≥ 74 (anti-gaming floor, ~90% of 82)   → C411_004 (pre-existing GREEN)
//	  AC5 5 gate anchors intact                                   → C411_005 (pre-existing GREEN, config-check)
//
//	tsc-compress-tdd-engineer-prompt:
//	  AC1 bytes < 21712 (≥15% cut from 25544)                    → C411_006 (RED)
//	  AC2 TSC marker present                                      → C411_007 (RED)
//	  AC3 header count == 17 exact                                → C411_008 (pre-existing GREEN, config-check)
//	  AC4 code-span lines ≥ 85 (anti-gaming floor, ~89% of 95)   → C411_009 (pre-existing GREEN)
//	  AC5 3 EGPS/ACS anchors intact                              → C411_010 (pre-existing GREEN, config-check)
//
//	tsc-compress-orchestrator-prompt:
//	  AC1 bytes < 20945 (≥15% cut from 24642)                    → C411_011 (RED)
//	  AC2 TSC marker present                                      → C411_012 (RED)
//	  AC3 header count == 26 exact                                → C411_013 (pre-existing GREEN, config-check)
//	  AC4 code-span lines ≥ 99 (anti-gaming floor, ~90% of 110)  → C411_014 (pre-existing GREEN)
//	  AC5 phase/guard anchors intact                             → C411_015 (pre-existing GREEN, config-check)
//
// Adversarial diversity (per SKILL §6):
//
//	Negative: unmodified file (above byte threshold) → C411_001/006/011 (RED)
//	Edge/OOD: file with code examples deleted → C411_004/009/014 (code-span floor)
//	Semantic:  TSC marker absent vs byte count are distinct failure modes
//
// Deferred (zero predicates per R9.3): D1 CompactPrompts strip path,
// D2 report-size tightening, D3 router/sidecars TSC.
```
