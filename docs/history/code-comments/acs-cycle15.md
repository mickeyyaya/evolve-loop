# Comment history: `acs/cycle15`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle15/predicates_test.go:3` — above `package cycle15`

```text
// Package cycle15 materializes the cycle-15 acceptance criteria for two
// committed top_n tasks:
//
//	consolidate-resume-cluster — remove all 6 RESUME_* registry rows
//	(EVOLVE_AUTO_RESUME_MAX_ATTEMPTS, EVOLVE_RESUME, EVOLVE_RESUME_ALLOW_HEAD_MOVED,
//	EVOLVE_RESUME_COMPLETED_PHASES, EVOLVE_RESUME_MODE, EVOLVE_RESUME_PHASE),
//	lower FlagCeiling 160→154, remove RESUME_* rows from control-flags.md,
//	and clean up docs_contract_test.go entries.
//
//	bypass-policy-flag — convert EVOLVE_POLICY_BYPASS to a proper --bypass-policy
//	cobra flag on `evolve cycle run` and `evolve loop`, remove 3 cycleEnv bridge
//	reads, and delete the EVOLVE_POLICY_BYPASS registry row.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-resume-cluster:
//	  AC1+NEG1  All 6 RESUME_* flags absent from Lookup             → C15_001 (behavioral)
//	  AC2       Registry row count == 154                            → C15_002 (behavioral, count)
//	  AC3       FlagCeiling const == 154                             → C15_003 (config-check, waiver)
//	  AC4       No os.Getenv reads for RESUME_* in production files  → C15_004 (config-check, waiver — PRE-EXISTING GREEN)
//	  AC5       control-flags.md has no RESUME_* rows               → C15_005 (config-check, waiver)
//	  EDGE1     IPC set preserved: cmd_loop_args.go sets EVOLVE_RESUME=1  → C15_006 (config-check, waiver — PRE-EXISTING GREEN)
//
//	bypass-policy-flag:
//	  AC1   --bypass-policy in `evolve cycle run --help`                   → C15_007 (behavioral, subprocess)
//	  AC2   bypass_policy field in `evolve loop --dry-run --bypass-policy` → C15_008 (behavioral, subprocess)
//	  AC3+4 cycleEnv["EVOLVE_POLICY_BYPASS"] absent from cmd files         → C15_009 (config-check, waiver)
//	  AC5   EVOLVE_POLICY_BYPASS row absent from flagregistry              → C15_010 (behavioral, Lookup)
//	  EDGE1 No os.Getenv("EVOLVE_POLICY_BYPASS") in production Go files    → PRE-EXISTING GREEN (0 production readers; scout confirmed)
//
// ACs with manual+checklist disposition (enforced by CI):
//
//	AC10  (full test suite green): `go test ./...` exit 0
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C15_001 (Lookup ok=false for all 6 RESUME_* rows) +
//	           C15_010 (Lookup ok=false for POLICY_BYPASS row) — neither
//	           can be satisfied by adding a magic string; the row must be deleted.
//	Edge/OOD:  C15_009 checks BOTH cmd_cycle.go AND cmd_loop.go (3 env bridge
//	           sites: line 190 in cycle, lines 186+303 in loop).
//	Lexical:   SubprocessOutput / FileNotContains / Lookup — three distinct verbs
//	           across the bypass-policy predicates.
//	Semantic:  CLI flag registration (C15_007), dry-run JSON field (C15_008),
//	           env-bridge absence (C15_009), registry-row absence (C15_010) —
//	           four distinct behavioral dimensions.
//
// 1:1 enforcement (bypass-policy-flag): predicate=4, manual+checklist=1, pre-existing-GREEN=1 → total AC=6 ✓
//
// Floor binding (R9.3): predicates authored only for the committed top_n tasks
// (consolidate-resume-cluster, bypass-policy-flag).
```
