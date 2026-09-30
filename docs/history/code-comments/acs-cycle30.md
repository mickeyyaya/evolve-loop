# Comment history: `acs/cycle30`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle30/predicates_test.go:3` — above `package cycle30`

```text
// Package cycle30 materializes the cycle-30 acceptance criteria for:
//
//	recovery-retry-config-cluster-30 — remove 6 EVOLVE_PHASE/RETRY/CONTRACT/SKIP
//	flags by migrating 4 into policy.RetryConfig (Configuration Object, cluster 8)
//	and dead-sweeping 2 comment-only flags:
//	  - EVOLVE_PHASE_MAX_ATTEMPTS         → RetryConfig.PhaseMaxAttempts (default 2)
//	  - EVOLVE_RETRY_BACKOFF_BASE_S       → RetryConfig.RetryBackoffBaseS (default 5)
//	  - EVOLVE_PHASE_LATENCY_CEILING_S    → RetryConfig.PhaseLatencyCeilingS (default 900)
//	  - EVOLVE_CONTRACT_CORRECTION_RETRIES → RetryConfig.ContractCorrectionRetries (default 2)
//	  - EVOLVE_PHASE_LATENCY_CEILING      → dead sweep (comment-only in cyclehealth.go:20)
//	  - EVOLVE_SKIP_CYCLE_HEALTH          → dead sweep (comment-only in cyclehealth.go:24)
//	Lower FlagCeiling 115→109; regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	recovery-retry-config-cluster-30:
//	  AC1  6 flags absent from Lookup              → C30_001 (behavioral)
//	  AC2  Registry row count == 109               → C30_002 (behavioral, count)
//	  AC3  FlagCeiling const == 109                → C30_003 (config-check, waiver)
//	  AC4  No envchain key constants in prod Go    → C30_004 (config-check, waiver)
//	  AC5  policy.RetryConfig struct + method      → C30_005 (behavioral + reflect)
//	  AC6  EVOLVE_WORKTREE_PATH still registered   → C30_006 (behavioral, PRE-EXISTING GREEN)
//	  AC7  flagreaders guard green                 → manual+checklist (see below)
//	  AC8  control-flags.md has no removed rows    → C30_008 (config-check, waiver)
//	  NEG1 Resolver fns deleted from retry_backoff → C30_NEG1 (config-check, waiver)
//	  NEG2 cyclehealth.go stops direct env read    → C30_NEG2 (config-check, waiver)
//
// ACs with manual+checklist disposition:
//
//	AC7 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle30 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no literal string "EVOLVE_PHASE_MAX_ATTEMPTS", "EVOLVE_RETRY_BACKOFF_BASE_S",
//	        "EVOLVE_PHASE_LATENCY_CEILING_S", or "EVOLVE_CONTRACT_CORRECTION_RETRIES"
//	        in any non-test, non-registry Go file
//	        (grep -rn 'EVOLVE_PHASE_MAX_ATTEMPTS\|EVOLVE_RETRY_BACKOFF_BASE_S\|
//	         EVOLVE_PHASE_LATENCY_CEILING_S\|EVOLVE_CONTRACT_CORRECTION_RETRIES'
//	         go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go'
//	        | grep -v 'acs/cycle30' → 0 matches);
//	    (d) EVOLVE_PHASE_LATENCY_CEILING and EVOLVE_SKIP_CYCLE_HEALTH are also absent
//	        from all production Go (they were comment-only; no env reads to clean up).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C30_001 — 6 flags must be ABSENT from Lookup (if Builder misses
//	           one, Lookup returns ok=true and the test fails immediately).
//	           C30_004 — envchain key constant definitions must be ABSENT from
//	           envchain/keys.go (if Builder only removes the registry row without
//	           deleting the key constant, the literal stays and this fails).
//	           C30_NEG1 — resolve functions must be ABSENT from retry_backoff.go
//	           (if Builder only removes the env read but leaves the resolve fn, this fails).
//	           C30_NEG2 — cyclehealth.go direct env read via KeyPhaseLatencyCeilingS
//	           must be ABSENT (if Builder only removes the registry row and key constant
//	           but forgets the cyclehealth.go call site, this fails).
//	Edge/OOD:  C30_002 checks exact count 109; both over-removal (< 109) and
//	           under-removal (> 109) fail.
//	Lexical:   Lookup / len / FileContains / FileNotContains / reflect —
//	           distinct assertion verbs across the suite.
//	Semantic:  registry-absence, row-count, ceiling-const, no-key-constants,
//	           retry-config-struct, worktree-path-present, doc-absence,
//	           resolver-fn-deleted, cyclehealth-env-read-deleted — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (recovery-retry-config-cluster-30). Deferred tasks (BYPASS cluster, etc.) get
// zero predicates.
//
// 1:1 enforcement: predicate=9, manual+checklist=1, unverifiable-remove=0 → total AC=10 ✓
```

### `go/acs/cycle30/predicates_test.go:84` — above `var retryFlags = []string{`

```text
// retryFlags is the canonical list of 6 flags that cycle-30 removes:
//   - EVOLVE_CONTRACT_CORRECTION_RETRIES: migrated to policy.RetryConfig.ContractCorrectionRetries
//   - EVOLVE_PHASE_LATENCY_CEILING:       dead sweep (comment-only, no Go reader)
//   - EVOLVE_PHASE_LATENCY_CEILING_S:     migrated to policy.RetryConfig.PhaseLatencyCeilingS
//   - EVOLVE_PHASE_MAX_ATTEMPTS:          migrated to policy.RetryConfig.PhaseMaxAttempts
//   - EVOLVE_RETRY_BACKOFF_BASE_S:        migrated to policy.RetryConfig.RetryBackoffBaseS
//   - EVOLVE_SKIP_CYCLE_HEALTH:           dead sweep (comment-only, no Go reader)
```

### `go/acs/cycle30/predicates_test.go:288` — above `func TestC30_006_WorktreePathStillRegistered(t *testing.T) {`

```text
// TestC30_006_WorktreePathStillRegistered is the non-repeat guard: verifies that
// EVOLVE_WORKTREE_PATH was NOT accidentally removed as part of the recovery-retry
// cluster sweep. Cycles 17, 18, and 19 all failed with FAIL (audit H1) when a
// Builder removed WORKTREE_PATH — this predicate closes that regression surface.
//
// Covers AC6 (FORBIDDEN-REPEAT guard). BEHAVIORAL: calls flagregistry.Lookup —
// the test fails if Builder removes the row (Lookup returns ok=false).
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently in the registry (StatusInternal);
// this test is GREEN before Builder makes any changes. It stays GREEN only if
// Builder does NOT touch the WORKTREE_PATH row.
```

### `go/acs/cycle30/predicates_test.go:333` — above `func TestC30_NEG1_NoResidualResolverFunctionsInRetryBackoff(t *testing.T) {`

```text
// TestC30_NEG1_NoResidualResolverFunctionsInRetryBackoff is the anti-gaming
// predicate that verifies retry_backoff.go no longer contains the three private
// resolver function definitions that previously wrapped the envchain reads.
//
// Anti-gaming rationale (cycle-8/cycle-85 lesson): a Builder could delete the
// registry rows and envchain constants while RENAMING (not deleting) the resolver
// functions, or leaving them as dead code. C30_004 catches the key-constant
// call sites; NEG1 adds a second layer by asserting the resolver function BODIES
// are gone — even if only renamed or stubbed — confirming the env-read logic was
// truly removed, not hidden.
//
// acs-predicate: config-check
//
// RED:
//
//	retry_backoff.go:18  defines "func resolvePhaseMaxAttempts("
//	retry_backoff.go:40  defines "func resolveContractCorrectionRetries("
//	retry_backoff.go:53  defines "func resolveRetryBackoffBase("
```
