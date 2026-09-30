# Comment history: `acs/cycle26`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle26/predicates_test.go:3` — above `package cycle26`

```text
// Package cycle26 materializes the cycle-26 acceptance criteria for:
//
//	quota-config-object — remove 3 flags (EVOLVE_ACS_PREDICATE_TIMEOUT_S dead
//	sweep, EVOLVE_QUOTA_RESET_AT, EVOLVE_QUOTA_RESET_HOURS) by migrating
//	quotareset.Options env reads to typed fields (ResetAt string, DefaultHours
//	float64) loaded via policy.json QuotaResetConfig. Lower FlagCeiling 132→129,
//	regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	quota-config-object:
//	  AC1  3 flags absent from Lookup              → C26_001 (behavioral)
//	  AC2  Registry row count == 129               → C26_002 (behavioral, count)
//	  AC3  FlagCeiling const == 129                → C26_003 (config-check, waiver)
//	  AC4  No env reads for QUOTA flags             → C26_004 (config-check, waiver)
//	  AC5  QuotaResetConfig field in policy.Policy  → C26_005 (behavioral + config-check, mixed)
//	  AC6  WORKTREE_PATH still registered           → C26_006 (behavioral — PRE-EXISTING GREEN)
//	  AC7  flagreaders guard green                  → manual+checklist (see below)
//	  AC8  control-flags.md has no removed rows    → C26_008 (config-check, waiver)
//	  NEG1 quotareset.Options typed fields + cmd   → C26_NEG1 (behavioral + config-check, mixed)
//	  NEG2 ACS_PREDICATE_TIMEOUT_S zero env reads  → C26_NEG2 (config-check, waiver — PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle26 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no stale EVOLVE_QUOTA_RESET_AT / EVOLVE_QUOTA_RESET_HOURS literal
//	        strings in non-test production Go (grep -rn
//	        'EVOLVE_QUOTA_RESET_AT\|EVOLVE_QUOTA_RESET_HOURS' go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C26_001 — flags must be ABSENT (if Builder removes wrong flags or
//	           misses one, Lookup returns ok=true and the test fails immediately).
//	           C26_004 — env reads must be ABSENT (FileNotContains on exact getEnv
//	           call strings; removing only the registry row without deleting the
//	           env reads leaves the literals in source and fails this test).
//	Edge/OOD:  C26_002 checks exact count 129; both over-removal (< 129) and
//	           under-removal (> 129) fail. C26_006 guards WORKTREE_PATH — the
//	           "over-removal" edge that killed cycles 17-19.
//	Lexical:   Lookup / len / FileContains / FileNotContains / reflect.FieldByName
//	           / SubprocessOutput — six distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-env-reads,
//	           typed-field-reflection, worktree-path-preserved, doc-absence,
//	           options-fields-reflection, reader-zero-grep — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (quota-config-object). Deferred tasks (WORKTREE_PATH, rollout-stages,
// workflow-defaults, BYPASS_* cluster) get zero predicates.
//
// 1:1 enforcement: predicate=9, manual+checklist=1, unverifiable-remove=0 → total AC=10 ✓
```

### `go/acs/cycle26/predicates_test.go:70` — above `var removedFlags = []string{`

```text
// removedFlags is the canonical list of 3 flags that cycle-26 removes:
//   - EVOLVE_ACS_PREDICATE_TIMEOUT_S: dead flag (comment-only in changedpkgs.go, zero env reads)
//   - EVOLVE_QUOTA_RESET_AT: migrated from quotareset.go:51 getEnv call to opts.ResetAt typed field
//   - EVOLVE_QUOTA_RESET_HOURS: migrated from quotareset.go:83 getEnv call to opts.DefaultHours typed field
```

### `go/acs/cycle26/predicates_test.go:192` — above `func TestC26_006_WorktreePathStillInRegistry(t *testing.T) {`

```text
// TestC26_006_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 3-row removal — it is a live IPC handoff
// (agents/evolve-tester.md) pinned by TestC50_009.
//
// Covers AC6 (WORKTREE_PATH must not be touched). Cycles 17, 18, and 19 all
// failed when Builder over-reached and removed WORKTREE_PATH, breaking TestC50_009.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered and must stay so.
```
