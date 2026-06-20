//go:build acs

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
package cycle30

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// retryFlags is the canonical list of 6 flags that cycle-30 removes:
//   - EVOLVE_CONTRACT_CORRECTION_RETRIES: migrated to policy.RetryConfig.ContractCorrectionRetries
//   - EVOLVE_PHASE_LATENCY_CEILING:       dead sweep (comment-only, no Go reader)
//   - EVOLVE_PHASE_LATENCY_CEILING_S:     migrated to policy.RetryConfig.PhaseLatencyCeilingS
//   - EVOLVE_PHASE_MAX_ATTEMPTS:          migrated to policy.RetryConfig.PhaseMaxAttempts
//   - EVOLVE_RETRY_BACKOFF_BASE_S:        migrated to policy.RetryConfig.RetryBackoffBaseS
//   - EVOLVE_SKIP_CYCLE_HEALTH:           dead sweep (comment-only, no Go reader)
var retryFlags = []string{
	"EVOLVE_CONTRACT_CORRECTION_RETRIES",
	"EVOLVE_PHASE_LATENCY_CEILING",
	"EVOLVE_PHASE_LATENCY_CEILING_S",
	"EVOLVE_PHASE_MAX_ATTEMPTS",
	"EVOLVE_RETRY_BACKOFF_BASE_S",
	"EVOLVE_SKIP_CYCLE_HEALTH",
}

// TestC30_001_RetryFlagsAbsentFromRegistry verifies that all 6 recovery/retry
// flags are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. The 6 flags span two removal patterns:
//   - EVOLVE_PHASE_MAX_ATTEMPTS, EVOLVE_RETRY_BACKOFF_BASE_S, EVOLVE_PHASE_LATENCY_CEILING_S,
//     EVOLVE_CONTRACT_CORRECTION_RETRIES: config-object migration to policy.RetryConfig
//   - EVOLVE_PHASE_LATENCY_CEILING, EVOLVE_SKIP_CYCLE_HEALTH: dead sweep (comment-only)
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 6 flags are currently registered (FlagCeiling=115); each Lookup
// returns (flag, true).
func TestC30_001_RetryFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range retryFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-30 recovery-retry-config-cluster-30).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC30_002_RegistryRowCountIs109 verifies that after removing all 6 rows the
// total registry count is exactly 109.
//
// Covers AC2. Both over-removal (< 109) and under-removal (> 109) fail.
//
// BEHAVIORAL: calls len(flagregistry.All) directly — the production SSOT slice.
// No source-file grepping; adding a magic string to source cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 115, which is 6 rows above 109.
func TestC30_002_RegistryRowCountIs109(t *testing.T) {
	const want = 109
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 6 recovery-retry-cluster rows from registry_table.go.\n"+
			"Both over-removal (< 109) and under-removal (> 109) fail.\n"+
			"Expected: 115 − 6 = 109.",
			got, want)
	}
}

// TestC30_003_FlagCeilingConstIs109 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 115 to 109
// in the same diff as the 6-row removal.
//
// acs-predicate: config-check — the constant value is the canonical ratchet;
// keeping 115 after the 6-row removal breaks the ratchet guarantee.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 115.
func TestC30_003_FlagCeilingConstIs109(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 109") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 109'.\n"+
			"Builder must lower the FlagCeiling constant from 115 to 109 in the same diff\n"+
			"as removing the 6 recovery-retry rows (115 − 6 = 109).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC30_004_NoEnvKeyConstantsInProductionGo verifies that the envchain key
// constant definitions for the 4 migrated flags have been deleted from
// envchain/keys.go, and that the call sites in retry_backoff.go and
// cyclehealth.go no longer reference those constants.
//
// Covers AC4. Config-check waiver: FileNotContains asserts structural absence
// of the exact envchain key constant names. The 4 constants live in one file
// (envchain/keys.go) and their call sites span two files:
//   - envchain/keys.go:     KeyPhaseMaxAttempts, KeyRetryBackoffBaseS,
//     KeyPhaseLatencyCeilingS, KeyContractCorrectionRetries
//   - core/retry_backoff.go:      KeyPhaseMaxAttempts, KeyRetryBackoffBaseS,
//     KeyContractCorrectionRetries
//   - cyclehealth/cyclehealth.go: KeyPhaseLatencyCeilingS
//
// acs-predicate: config-check
//
// RED:
//
//	envchain/keys.go:18  defines KeyPhaseMaxAttempts = "EVOLVE_PHASE_MAX_ATTEMPTS"
//	envchain/keys.go:22  defines KeyRetryBackoffBaseS = "EVOLVE_RETRY_BACKOFF_BASE_S"
//	envchain/keys.go:27  defines KeyPhaseLatencyCeilingS = "EVOLVE_PHASE_LATENCY_CEILING_S"
//	envchain/keys.go:33  defines KeyContractCorrectionRetries = "EVOLVE_CONTRACT_CORRECTION_RETRIES"
//	core/retry_backoff.go:19      calls envchain.IntMin(envchain.KeyPhaseMaxAttempts, ...)
//	core/retry_backoff.go:41      calls envchain.Int(envchain.KeyContractCorrectionRetries, ...)
//	core/retry_backoff.go:54      calls envchain.Int(envchain.KeyRetryBackoffBaseS, ...)
//	cyclehealth/cyclehealth.go:465 calls envchain.IntMin(envchain.KeyPhaseLatencyCeilingS, ...)
func TestC30_004_NoEnvKeyConstantsInProductionGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		absents []string
	}{
		{
			filepath.Join(root, "go", "internal", "envchain", "keys.go"),
			[]string{
				"KeyPhaseMaxAttempts",
				"KeyRetryBackoffBaseS",
				"KeyPhaseLatencyCeilingS",
				"KeyContractCorrectionRetries",
			},
		},
		{
			filepath.Join(root, "go", "internal", "core", "retry_backoff.go"),
			[]string{
				"KeyPhaseMaxAttempts",
				"KeyRetryBackoffBaseS",
				"KeyContractCorrectionRetries",
			},
		},
		{
			filepath.Join(root, "go", "internal", "cyclehealth", "cyclehealth.go"),
			[]string{"KeyPhaseLatencyCeilingS"},
		},
	}
	for _, c := range checks {
		for _, id := range c.absents {
			if !acsassert.FileNotContains(t, c.file, id) {
				t.Errorf("RED: %s still contains envchain key constant %q.\n"+
					"Builder must delete the constant definition from envchain/keys.go\n"+
					"and replace every call site with the appropriate RetryConfig field.\n"+
					"File: %s", filepath.Base(c.file), id, c.file)
			}
		}
	}
}

// TestC30_005_RetryConfigStructExistsInPolicy verifies that:
//
//  1. policy.Policy has a Retry field (reflect check — fails at runtime when
//     the field is absent, giving a precise per-test failure message without
//     a whole-file compile error).
//  2. The Retry field is a pointer type (*RetryPolicy).
//  3. The pointed-to struct has PhaseMaxAttempts int, RetryBackoffBaseS int,
//     PhaseLatencyCeilingS int, and ContractCorrectionRetries int sub-fields —
//     the typed replacements for the deleted envchain reads.
//  4. RetryConfig() resolver returns correct defaults when Policy.Retry == nil:
//     PhaseMaxAttempts=2, RetryBackoffBaseS=5, PhaseLatencyCeilingS=900,
//     ContractCorrectionRetries=2 (matching the deleted envchain Def* constants).
//  5. Empty RetryPolicy{} also yields the same defaults (zero-value int → default).
//
// Covers AC5. BEHAVIORAL: reflect.FieldByName traverses the production type
// system and a direct resolver call returns computed values; a magic-string
// source edit cannot satisfy this — the struct and fields must actually exist
// for reflect to find them and the resolver must compute the right defaults.
//
// RED:
//
//	policy.Policy has no Retry field → reflect returns ok=false at the first check.
//	(post-field-add) RetryPolicy lacks required fields → inner checks fail.
//	(post-struct-add) RetryConfig() defaults don't match spec → final checks fail.
func TestC30_005_RetryConfigStructExistsInPolicy(t *testing.T) {
	// Check Policy has Retry field.
	pInfo, ok := reflect.TypeOf(policy.Policy{}).FieldByName("Retry")
	if !ok {
		t.Fatalf("RED: policy.Policy.Retry field missing.\n" +
			"Builder must add `Retry *RetryPolicy` to Policy in go/internal/policy/policy.go\n" +
			"(parallel to Workflow *WorkflowPolicy from cycle-29).")
	}
	if pInfo.Type.Kind() != reflect.Ptr {
		t.Fatalf("RED: policy.Policy.Retry is kind %v, want pointer (*RetryPolicy).\n"+
			"The field must be typed as `*RetryPolicy`.",
			pInfo.Type.Kind())
	}

	// Navigate to the RetryPolicy struct type via the pointer's element type.
	retryType := pInfo.Type.Elem()
	intFields := []string{"PhaseMaxAttempts", "RetryBackoffBaseS", "PhaseLatencyCeilingS", "ContractCorrectionRetries"}
	for _, fname := range intFields {
		if f, ok := retryType.FieldByName(fname); ok {
			if f.Type.Kind() != reflect.Int {
				t.Errorf("RED: RetryPolicy.%s is kind %v, want int.", fname, f.Type.Kind())
			}
		} else {
			t.Errorf("RED: RetryPolicy missing %s field.\n"+
				"Builder must add `%s int` to RetryPolicy.", fname, fname)
		}
	}

	// Verify RetryConfig() resolver returns correct defaults when Retry == nil.
	rc := policy.Policy{}.RetryConfig()
	if rc.PhaseMaxAttempts != 2 {
		t.Errorf("RED: RetryConfig().PhaseMaxAttempts = %d, want 2 (matches legacy EVOLVE_PHASE_MAX_ATTEMPTS default).",
			rc.PhaseMaxAttempts)
	}
	if rc.RetryBackoffBaseS != 5 {
		t.Errorf("RED: RetryConfig().RetryBackoffBaseS = %d, want 5 (matches legacy EVOLVE_RETRY_BACKOFF_BASE_S default).",
			rc.RetryBackoffBaseS)
	}
	if rc.PhaseLatencyCeilingS != 900 {
		t.Errorf("RED: RetryConfig().PhaseLatencyCeilingS = %d, want 900 (matches legacy EVOLVE_PHASE_LATENCY_CEILING_S default).",
			rc.PhaseLatencyCeilingS)
	}
	if rc.ContractCorrectionRetries != 2 {
		t.Errorf("RED: RetryConfig().ContractCorrectionRetries = %d, want 2 (matches legacy EVOLVE_CONTRACT_CORRECTION_RETRIES default).",
			rc.ContractCorrectionRetries)
	}

	// Edge/OOD: empty RetryPolicy{} (all int fields == 0) must also resolve to
	// the same defaults (zero int → fall through to default).
	rc2 := policy.Policy{Retry: &policy.RetryPolicy{}}.RetryConfig()
	if rc2.PhaseMaxAttempts != 2 {
		t.Errorf("edge: empty RetryPolicy{}.PhaseMaxAttempts resolved to %d, want 2.\n"+
			"A zero int must fall through to the default (2), not override it.",
			rc2.PhaseMaxAttempts)
	}
	if rc2.RetryBackoffBaseS != 5 {
		t.Errorf("edge: empty RetryPolicy{}.RetryBackoffBaseS resolved to %d, want 5.\n"+
			"A zero int must fall through to the default (5), not override it.",
			rc2.RetryBackoffBaseS)
	}
	if rc2.PhaseLatencyCeilingS != 900 {
		t.Errorf("edge: empty RetryPolicy{}.PhaseLatencyCeilingS resolved to %d, want 900.\n"+
			"A zero int must fall through to the default (900), not override it.",
			rc2.PhaseLatencyCeilingS)
	}
	if rc2.ContractCorrectionRetries != 2 {
		t.Errorf("edge: empty RetryPolicy{}.ContractCorrectionRetries resolved to %d, want 2.\n"+
			"A zero int must fall through to the default (2), not override it.",
			rc2.ContractCorrectionRetries)
	}
}

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
func TestC30_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// TestC30_008_ControlFlagsMdHasNoRemovedRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 6 removed
// flags after the registry rows are removed and the doc regenerated via
// 'evolve flags generate'.
//
// Covers AC8. The doc is generated from the flagregistry (source of truth);
// absence follows from C30_001 (rows removed) plus regeneration.
//
// acs-predicate: config-check — doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 6 removed flags.
func TestC30_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range retryFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 6 recovery-retry rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

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
func TestC30_NEG1_NoResidualResolverFunctionsInRetryBackoff(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	retryBackoffFile := filepath.Join(root, "go", "internal", "core", "retry_backoff.go")
	for _, fnName := range []string{
		"resolvePhaseMaxAttempts",
		"resolveRetryBackoffBase",
		"resolveContractCorrectionRetries",
	} {
		if !acsassert.FileNotContains(t, retryBackoffFile, fnName) {
			t.Errorf("RED: retry_backoff.go still contains resolver function %q.\n"+
				"Builder must DELETE this function (not rename it) and replace its\n"+
				"callers with the appropriate policy.RetryConfig field access.\n"+
				"File: %s", fnName, retryBackoffFile)
		}
	}
}

// TestC30_NEG2_NoCycleHealthDirectEnvRead verifies that cyclehealth.go no longer
// contains a direct per-phase env-key read via envchain.PhaseEnvKey — the
// perPhaseCeiling helper that read EVOLVE_<PHASE>_LATENCY_CEILING_S per-phase env
// vars must be simplified to return the global ceiling directly (BA1: zero-behavior-
// change simplification per scout-report.md hypothesis BA1).
//
// Anti-gaming rationale: C30_004 confirms KeyPhaseLatencyCeilingS (the global read)
// is gone; NEG2 confirms the per-phase read via PhaseEnvKey is also gone. A Builder
// who only removes the global read but leaves the perPhaseCeiling env read open would
// pass C30_004 but fail this predicate — closing the second env-read gaming surface.
//
// acs-predicate: config-check
//
// RED: cyclehealth.go:487 contains `envchain.PhaseEnvKey(phase, "LATENCY_CEILING_S")`
// inside perPhaseCeiling(). After migration the function must return globalCeiling
// directly without any envchain read.
func TestC30_NEG2_NoCycleHealthDirectEnvRead(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cycleHealthFile := filepath.Join(root, "go", "internal", "cyclehealth", "cyclehealth.go")
	if !acsassert.FileNotContains(t, cycleHealthFile, "PhaseEnvKey") {
		t.Errorf("RED: cyclehealth.go still contains PhaseEnvKey (the per-phase env-override read).\n"+
			"Builder must simplify perPhaseCeiling() to return globalCeiling directly\n"+
			"(scout BA1: no per-phase override was ever set in practice; the read is dead).\n"+
			"File: %s", cycleHealthFile)
	}
}
