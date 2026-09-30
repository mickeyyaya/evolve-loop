# Comment history: `acs/cycle32`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/acs/cycle32/predicates_test.go:3` — above `package cycle32`

```text
// Package cycle32 materializes the cycle-32 acceptance criteria for:
//
//	workflow-internal-cluster-32 — migrate 4 live EVOLVE_* flags from os.Getenv/
//	envchain/envEnabled reads to policy.WorkflowConfig (Configuration Object + DI);
//	dead-sweep EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S (zero Go reader). 5 flags:
//	  - EVOLVE_BACKFILL_ENABLED        → WorkflowConfig.BackfillEnabled *bool (cyclerun_dispatch)
//	  - EVOLVE_CYCLE_BUDGET            → WorkflowConfig.CycleBudget string (cmd_loop)
//	  - EVOLVE_ALLOW_DEEP_RESEARCH     → QuotaConfig.AllowDeepResearch bool (DI in guards/quota)
//	  - EVOLVE_ALLOW_DOC_DELETE        → DocDelete.allow bool (DI in guards/docdelete)
//	  - EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S → dead sweep (docs-only; no Go reader)
//	Lower FlagCeiling 102→97; regenerate docs/architecture/control-flags.md.
//	Delete envEnabled() helper from guards/helpers.go (zero callers after migration).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	workflow-internal-cluster-32:
//	  AC1   5 flags absent from Lookup                → C32_001 (behavioral)
//	  AC2   Registry row count == 97                  → C32_002 (behavioral, count)
//	  AC3   FlagCeiling const == 97                   → C32_003 (config-check, waiver)
//	  AC4   No env reads for 4 migrated flags in prod → C32_004 (config-check, waiver)
//	  AC5   WorkflowConfig().BackfillEnabled==true,   → C32_005 (behavioral, direct Go call)
//	         AllowDeepResearch==false (defaults)
//	  EDGE1 nil *bool BackfillEnabled → true default  → (covered by C32_005)
//	  AC6   EVOLVE_WORKTREE_PATH still registered     → C32_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7   flagreaders regression guard green         → manual+checklist (see below)
//	  AC8   control-flags.md has no removed flag rows → C32_008 (config-check, waiver)
//	  NEG1  envEnabled helper deleted from helpers.go → C32_NEG1 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle32 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_BACKFILL_ENABLED", "EVOLVE_CYCLE_BUDGET",
//	        "EVOLVE_ALLOW_DEEP_RESEARCH", or "EVOLVE_ALLOW_DOC_DELETE"
//	        in any non-test, non-registry Go file via os.Getenv, envchain, or envEnabled
//	        (grep -rn 'os\.Getenv.*EVOLVE_BACKFILL_ENABLED\|EVOLVE_CYCLE_BUDGET\|
//	         EVOLVE_ALLOW_DEEP_RESEARCH\|EVOLVE_ALLOW_DOC_DELETE'
//	         go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go'
//	        | grep -v 'acs/cycle32' → 0 matches);
//	    (d) EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S had zero Go readers before dead
//	        sweep; verify no new reader was added (grep returns 0 non-test Go matches).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C32_001 — 5 flags must be ABSENT from Lookup (if Builder misses any
//	           one, Lookup returns ok=true and the test fails immediately).
//	           C32_004 — env-read literals must be ABSENT from specific production
//	           files (if Builder removes registry rows without deleting call sites,
//	           the literal strings remain and this fails).
//	           C32_NEG1 — envEnabled() function itself must be ABSENT (if Builder
//	           migrates callers but leaves the dead helper, this fails — the cycle-85
//	           grep-only gaming surface closed at the function level).
//	Edge/OOD:  C32_002 checks exact count 97; both over-removal (< 97) and
//	           under-removal (> 97) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / direct struct field
//	           access — distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           config-defaults, worktree-path-present, doc-absence,
//	           helper-fn-deleted — 8 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (workflow-internal-cluster-32). Deferred tasks (PSMAS_SKIP, STRICT_AUDIT,
// STRATEGY, CONSENSUS_AUDIT) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=8, manual+checklist=1, unverifiable-remove=0 → total AC=9 ✓
//	(EDGE1 merged into C32_005; both default behaviors covered by one predicate)
```

### `go/acs/cycle32/predicates_test.go:85` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 5 flags that cycle-32 removes:
//   - EVOLVE_BACKFILL_ENABLED:               migrated to WorkflowConfig.BackfillEnabled
//   - EVOLVE_CYCLE_BUDGET:                   migrated to WorkflowConfig.CycleBudget
//   - EVOLVE_ALLOW_DEEP_RESEARCH:            migrated to QuotaConfig.AllowDeepResearch (DI)
//   - EVOLVE_ALLOW_DOC_DELETE:               migrated to DocDelete.allow (DI)
//   - EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S: dead sweep (docs-only; no Go reader)
```

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle32/predicates_test.go:3` — above `package cycle32`

```text
// Package cycle32 materializes the cycle-32 acceptance criteria for:
//
//	workflow-internal-cluster-32 — migrate 4 live EVOLVE_* flags from os.Getenv/
//	envchain/envEnabled reads to policy.WorkflowConfig (Configuration Object + DI);
//	dead-sweep EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S (zero Go reader). 5 flags:
//	  - EVOLVE_BACKFILL_ENABLED        → WorkflowConfig.BackfillEnabled *bool (cyclerun_dispatch)
//	  - EVOLVE_CYCLE_BUDGET            → WorkflowConfig.CycleBudget string (cmd_loop)
//	  - EVOLVE_ALLOW_DEEP_RESEARCH     → removed with the quota guard (2026-09-26)
//	  - EVOLVE_ALLOW_DOC_DELETE        → DocDelete.allow bool (DI in guards/docdelete)
//	  - EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S → dead sweep (docs-only; no Go reader)
//	Lower FlagCeiling 102→97; regenerate docs/architecture/control-flags.md.
//	Delete envEnabled() helper from guards/helpers.go (zero callers after migration).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	workflow-internal-cluster-32:
//	  AC1   5 flags absent from Lookup                → C32_001 (behavioral)
//	  AC2   Registry row count == 97                  → C32_002 (behavioral, count)
//	  AC3   FlagCeiling const == 97                   → C32_003 (config-check, waiver)
//	  AC4   No env reads for 4 migrated flags in prod → C32_004 (config-check, waiver)
//	  AC5   WorkflowConfig().BackfillEnabled==true,   → C32_005 (behavioral, direct Go call)
//	         AllowDeepResearch==false (defaults)
//	  EDGE1 nil *bool BackfillEnabled → true default  → (covered by C32_005)
//	  AC6   EVOLVE_WORKTREE_PATH still registered     → C32_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7   flagreaders regression guard green         → manual+checklist (see below)
//	  AC8   control-flags.md has no removed flag rows → C32_008 (config-check, waiver)
//	  NEG1  envEnabled helper deleted from helpers.go → C32_NEG1 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle32 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_BACKFILL_ENABLED", "EVOLVE_CYCLE_BUDGET",
//	        "EVOLVE_ALLOW_DEEP_RESEARCH", or "EVOLVE_ALLOW_DOC_DELETE"
//	        in any non-test, non-registry Go file via os.Getenv, envchain, or envEnabled
//	        (grep -rn 'os\.Getenv.*EVOLVE_BACKFILL_ENABLED\|EVOLVE_CYCLE_BUDGET\|
//	         EVOLVE_ALLOW_DEEP_RESEARCH\|EVOLVE_ALLOW_DOC_DELETE'
//	         go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go'
//	        | grep -v 'acs/cycle32' → 0 matches);
//	    (d) EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S had zero Go readers before dead
//	        sweep; verify no new reader was added (grep returns 0 non-test Go matches).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C32_001 — 5 flags must be ABSENT from Lookup (if Builder misses any
//	           one, Lookup returns ok=true and the test fails immediately).
//	           C32_004 — env-read literals must be ABSENT from specific production
//	           files (if Builder removes registry rows without deleting call sites,
//	           the literal strings remain and this fails).
//	           C32_NEG1 — envEnabled() function itself must be ABSENT (if Builder
//	           migrates callers but leaves the dead helper, this fails — the cycle-85
//	           grep-only gaming surface closed at the function level).
//	Edge/OOD:  C32_002 checks exact count 97; both over-removal (< 97) and
//	           under-removal (> 97) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / direct struct field
//	           access — distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           config-defaults, worktree-path-present, doc-absence,
//	           helper-fn-deleted — 8 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (workflow-internal-cluster-32). Deferred tasks (PSMAS_SKIP, STRICT_AUDIT,
// STRATEGY, CONSENSUS_AUDIT) get zero predicates.
//
// 1:1 enforcement:
//
//	predicate=8, manual+checklist=1, unverifiable-remove=0 → total AC=9 ✓
//	(EDGE1 merged into C32_005; both default behaviors covered by one predicate)
```

### `go/acs/cycle32/predicates_test.go:85` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 5 flags that cycle-32 removes:
//   - EVOLVE_BACKFILL_ENABLED:               migrated to WorkflowConfig.BackfillEnabled
//   - EVOLVE_CYCLE_BUDGET:                   migrated to WorkflowConfig.CycleBudget
//   - EVOLVE_ALLOW_DEEP_RESEARCH:            removed with the quota guard (2026-09-26)
//   - EVOLVE_ALLOW_DOC_DELETE:               migrated to DocDelete.allow (DI)
//   - EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S: dead sweep (docs-only; no Go reader)
```

### `go/acs/cycle32/predicates_test.go:200` — above `func TestC32_006_WorktreePathStillRegistered(t *testing.T) {`

```text
// TestC32_006_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the cluster sweep.
// Cycles 17, 18, and 19 all failed when a Builder removed WORKTREE_PATH —
// this predicate closes that regression surface.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
```

### `go/acs/cycle32/predicates_test.go:245` — above `func TestC32_NEG1_EnvEnabledHelperDeleted(t *testing.T) {`

```text
// TestC32_NEG1_EnvEnabledHelperDeleted is the anti-gaming predicate that verifies
// the envEnabled() helper function has been completely deleted from guards/helpers.go.
//
// Anti-gaming rationale (cycle-8/cycle-85 lesson): a Builder could migrate both
// quota.go and docdelete.go callers away from envEnabled() while leaving the dead
// helper function in place. C32_004 confirms the ALLOW_DEEP_RESEARCH and
// ALLOW_DOC_DELETE env-var strings are gone from the individual guard files;
// NEG1 adds a second layer by asserting the shared envEnabled() helper itself is
// gone — closing the gaming surface where the function stays as dead code.
//
// acs-predicate: config-check
//
// RED: guards/helpers.go:29 defines "func envEnabled(name string) bool" that
// reads os.Getenv(name) == "1". After migration, this function must not exist.
```
