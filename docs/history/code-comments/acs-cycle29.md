# Comment history: `acs/cycle29`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle29/predicates_test.go:3` — above `package cycle29`

```text
// Package cycle29 materializes the cycle-29 acceptance criteria for:
//
//	workflow-config-cluster-29 — remove 5 EVOLVE_WORKFLOW_DEFAULTS flags by
//	migrating them to policy.WorkflowConfig (Configuration Object, bucket 1):
//	  - EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS → WorkflowConfig.MaxConsecutiveFails (default 1)
//	  - EVOLVE_MAX_CYCLES_CAP            → WorkflowConfig.MaxCyclesCap (default 25)
//	  - EVOLVE_AUTO_PRUNE                → WorkflowConfig.AutoPrune (default true)
//	  - EVOLVE_DIFF_COMPLEXITY_DISABLE   → WorkflowConfig.DiffComplexityDisable (default false)
//	  - EVOLVE_AUDITOR_TIER_OVERRIDE     → WorkflowConfig.AuditorTierOverride (default "")
//	Lower FlagCeiling 120→115; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	workflow-config-cluster-29:
//	  AC1  5 flags absent from Lookup              → C29_001 (behavioral)
//	  AC2  Registry row count == 115               → C29_002 (behavioral, count)
//	  AC3  FlagCeiling const == 115                → C29_003 (config-check, waiver)
//	  AC4  No env reads for 5 flags in prod Go     → C29_004 (config-check, waiver)
//	  AC5  policy.WorkflowConfig struct + method   → C29_005 (behavioral + reflect)
//	  AC6  flagreaders guard green                 → manual+checklist (see below)
//	  AC7  EVOLVE_WORKTREE_PATH still registered   → C29_007 (behavioral, PRE-EXISTING GREEN)
//	  NEG1 Raising FlagCeiling breaks ratchet test → unverifiable-remove (structural guarantee)
//	  E01  Empty WorkflowPolicy{} → correct defaults → C29_E01 (behavioral)
//
// ACs with manual+checklist disposition:
//
//	AC6 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle29 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS",
//	        "EVOLVE_MAX_CYCLES_CAP", "EVOLVE_AUTO_PRUNE",
//	        "EVOLVE_DIFF_COMPLEXITY_DISABLE", or "EVOLVE_AUDITOR_TIER_OVERRIDE"
//	        in any non-test, non-registry Go file
//	        (grep -rn 'EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS\|EVOLVE_MAX_CYCLES_CAP\|
//	         EVOLVE_AUTO_PRUNE\|EVOLVE_DIFF_COMPLEXITY_DISABLE\|
//	         EVOLVE_AUDITOR_TIER_OVERRIDE' go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go'
//	        | grep -v 'acs/cycle29' → 0 matches);
//	    (d) the cmd_subagent.go usage string (Honors line) may retain a reference
//	        to EVOLVE_AUDITOR_TIER_OVERRIDE and EVOLVE_DIFF_COMPLEXITY_DISABLE as
//	        prose descriptions — this is acceptable (non-functional, purely doc).
//
// ACs with unverifiable-remove disposition:
//
//	NEG1: The ratchet guarantee (raising FlagCeiling above 115 causes test failure)
//	is structurally enforced by TestRegistry_FlagCeiling in registry_ceiling_test.go.
//	That test exists in the non-ACS normal suite and enforces the invariant
//	deterministically — no additional ACS predicate is needed or meaningful.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C29_001 — 5 flags must be ABSENT from Lookup (if Builder misses
//	           one, Lookup returns ok=true and the test fails immediately).
//	           C29_004 — env-read literals must be ABSENT from production source
//	           (if Builder only removes the registry row without deleting the env
//	           read, the literal stays and this fails).
//	Edge/OOD:  C29_002 checks exact count 115; both over-removal (< 115) and
//	           under-removal (> 115) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / reflect —
//	           distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           workflow-config-struct, flagreaders (manual), worktree-path-present,
//	           empty-policy-defaults — 8 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (workflow-config-cluster-29). Deferred tasks (STRICT_AUDIT cluster, BYPASS
// cluster, etc.) get zero predicates.
//
// 1:1 enforcement: predicate=7, manual+checklist=1, unverifiable-remove=1 → total AC=9 ✓
```

### `go/acs/cycle29/predicates_test.go:85` — above `var workflowFlags = []string{`

```text
// workflowFlags is the canonical list of 5 flags that cycle-29 removes by
// migrating them into policy.WorkflowConfig (Configuration Object pattern):
//   - EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS: resolved to WorkflowConfig.MaxConsecutiveFails
//   - EVOLVE_MAX_CYCLES_CAP:             resolved to WorkflowConfig.MaxCyclesCap
//   - EVOLVE_AUTO_PRUNE:                 resolved to WorkflowConfig.AutoPrune
//   - EVOLVE_DIFF_COMPLEXITY_DISABLE:    resolved to WorkflowConfig.DiffComplexityDisable
//   - EVOLVE_AUDITOR_TIER_OVERRIDE:      resolved to WorkflowConfig.AuditorTierOverride
```

### `go/acs/cycle29/predicates_test.go:268` — above `func TestC29_007_WorktreePathStillRegistered(t *testing.T) {`

```text
// TestC29_007_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the workflow-defaults
// cluster sweep. Cycles 17 and 18 both failed with FAIL (audit H1) when a Builder
// removed WORKTREE_PATH — this predicate closes that regression surface.
//
// Covers AC7 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
```
