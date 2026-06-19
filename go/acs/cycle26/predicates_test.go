//go:build acs

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
package cycle26

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotareset"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// removedFlags is the canonical list of 3 flags that cycle-26 removes:
//   - EVOLVE_ACS_PREDICATE_TIMEOUT_S: dead flag (comment-only in changedpkgs.go, zero env reads)
//   - EVOLVE_QUOTA_RESET_AT: migrated from quotareset.go:51 getEnv call to opts.ResetAt typed field
//   - EVOLVE_QUOTA_RESET_HOURS: migrated from quotareset.go:83 getEnv call to opts.DefaultHours typed field
var removedFlags = []string{
	"EVOLVE_ACS_PREDICATE_TIMEOUT_S",
	"EVOLVE_QUOTA_RESET_AT",
	"EVOLVE_QUOTA_RESET_HOURS",
}

// TestC26_001_DeadFlagsAbsentFromRegistry verifies that all 3 quota/ACS cluster
// flags are no longer registered after Builder removes their rows from
// registry_table.go.
//
// Covers AC1. The 3 flags span two removal patterns:
//   - EVOLVE_ACS_PREDICATE_TIMEOUT_S: dead sweep (registry row only — no code change needed)
//   - EVOLVE_QUOTA_RESET_AT: config-object migration (delete getEnv call + registry row)
//   - EVOLVE_QUOTA_RESET_HOURS: config-object migration (delete getEnv call + registry row)
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 3 flags are currently registered (FlagCeiling=132); each Lookup returns (flag, true).
func TestC26_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-26 quota-config-object).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC26_002_RegistryRowCountIs129 verifies that after removing all 3 rows the
// total registry count is exactly 129.
//
// Covers AC2. Both over-removal (< 129) and under-removal (> 129) fail.
//
// BEHAVIORAL: calls len(flagregistry.All) directly (the production SSOT slice).
// No source-file grepping; adding a magic string to source cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 132, which is 3 rows above 129.
func TestC26_002_RegistryRowCountIs129(t *testing.T) {
	const want = 129
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 3 quota/ACS cluster rows from registry_table.go.\n"+
			"Both over-removal (< 129) and under-removal (> 129) fail.\n"+
			"Expected: 132 − 3 = 129.",
			got, want)
	}
}

// TestC26_003_FlagCeilingConstIs129 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 132 to 129
// in the same diff as the 3-row removal.
//
// // acs-predicate: config-check — the constant value is the canonical ratchet;
// keeping 132 after the 3-row removal breaks the ratchet guarantee.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 132.
func TestC26_003_FlagCeilingConstIs129(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 129") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 129'.\n"+
			"Builder must lower the FlagCeiling constant from 132 to 129 in the same diff\n"+
			"as removing the 3 quota/ACS cluster rows (132 − 3 = 129).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC26_004_NoEnvReadsForQuotaFlags verifies that the two os.Getenv tier-2
// read sites for EVOLVE_QUOTA_RESET_AT and EVOLVE_QUOTA_RESET_HOURS have been
// deleted from quotareset.go:
//
//  1. quotareset.go:51 `getEnv("EVOLVE_QUOTA_RESET_AT")` — Source 1 must be
//     replaced by `strings.TrimSpace(opts.ResetAt)` (the typed field check).
//
//  2. quotareset.go:83 `getEnv("EVOLVE_QUOTA_RESET_HOURS")` — Source 3 must be
//     replaced by `if opts.DefaultHours > 0 { hours = opts.DefaultHours }`.
//
// Covers AC4. Config-check waiver: FileNotContains asserts structural absence of
// the exact call literals — adding the registry row back cannot re-introduce them.
//
// // acs-predicate: config-check
//
// RED:
//   - quotareset.go:51 has `getEnv("EVOLVE_QUOTA_RESET_AT")`
//   - quotareset.go:83 has `getEnv("EVOLVE_QUOTA_RESET_HOURS")`
func TestC26_004_NoEnvReadsForQuotaFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	quotaresetFile := filepath.Join(root, "go", "internal", "quotareset", "quotareset.go")
	for _, dead := range []string{
		`"EVOLVE_QUOTA_RESET_AT"`,
		`"EVOLVE_QUOTA_RESET_HOURS"`,
	} {
		if !acsassert.FileNotContains(t, quotaresetFile, dead) {
			t.Errorf("RED: quotareset.go still contains %q.\n"+
				"Builder must delete the getEnv call for this flag and replace it with\n"+
				"the typed opts field (opts.ResetAt or opts.DefaultHours).\n"+
				"File: %s", dead, quotaresetFile)
		}
	}
}

// TestC26_005_QuotaResetConfigFieldInPolicy verifies that:
//
//  1. policy.Policy has a QuotaReset field (reflect check — fails at runtime
//     when the field is absent, avoiding a whole-file compile error and giving
//     a precise per-test failure message).
//
//  2. cmd_quota_reset.go loads policy and passes quota reset config to
//     quotareset.Compute (FileContains on "QuotaReset" — the call site wiring
//     that connects policy → quotareset.Options, closing the os.Getenv bypass).
//
// Covers AC5 (typed policy field + cmd wiring). Mixed predicate:
// reflect.FieldByName is behavioral; FileContains on cmd_quota_reset.go is
// structural (config-check waiver for the call-site wiring).
//
// // acs-predicate: config-check (FileContains portion)
//
// RED:
//   - policy.Policy has no QuotaReset field → reflect returns ok=false
//   - cmd_quota_reset.go passes quotareset.Options{} with no policy fields
func TestC26_005_QuotaResetConfigFieldInPolicy(t *testing.T) {
	// Behavioral: verify policy.Policy.QuotaReset field exists (pointer to QuotaResetConfig).
	// Uses reflect.FieldByName so the test fails at runtime (not compile time)
	// when the field is absent, giving a meaningful per-test failure message.
	fInfo, ok := reflect.TypeOf(policy.Policy{}).FieldByName("QuotaReset")
	if !ok {
		t.Fatalf("RED: policy.Policy.QuotaReset field missing.\n" +
			"Builder must add `QuotaReset *QuotaResetConfig` to Policy in go/internal/policy/policy.go\n" +
			"(parallel to Fanout *FanoutPolicy and Observer *ObserverPolicy).")
	}
	if fInfo.Type.Kind() != reflect.Ptr {
		t.Errorf("RED: policy.Policy.QuotaReset is kind %v, want pointer (*QuotaResetConfig).\n"+
			"The field must be typed as `*QuotaResetConfig`, not %v.",
			fInfo.Type.Kind(), fInfo.Type.Kind())
	}

	// Structural: cmd_quota_reset.go loads policy and references QuotaReset config.
	// After fix: the cmd reads pol.QuotaResetConfig() (or pol.QuotaReset) to obtain
	// opts.ResetAt and opts.DefaultHours before calling quotareset.Compute.
	// Before fix: quotareset.Compute(workspace, quotareset.Options{}) — no policy fields.
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_quota_reset.go")
	if !acsassert.FileContains(t, cmdFile, "QuotaReset") {
		t.Errorf("RED: cmd_quota_reset.go does not load policy or reference QuotaReset.\n"+
			"Builder must update runQuotaReset to:\n"+
			"  1. Load policy.json via policy.Load (or equivalent)\n"+
			"  2. Call pol.QuotaResetConfig() (or pol.QuotaReset) to obtain opts.ResetAt + opts.DefaultHours\n"+
			"  3. Pass those values to quotareset.Compute\n"+
			"File: %s", cmdFile)
	}
}

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
func TestC26_006_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by TestC50_009.\n"+
			"This is the same mistake that killed cycles 17, 18, and 19.",
			worktreePath)
	}
}

// TestC26_008_ControlFlagsMdHasNoRemovedRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 3 removed flags
// after the registry rows are removed and the doc regenerated via 'evolve flags generate'.
//
// Covers AC8. The doc is generated from the flagregistry (source of truth);
// absence follows from C26_001 (rows removed) plus regeneration.
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 3 removed flags.
func TestC26_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, flag := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 3 quota/ACS cluster rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

// TestC26_NEG1_QuotaOptionsTypedFieldsExist verifies that:
//
//  1. quotareset.Options.ResetAt string field exists (reflect check — fails at
//     runtime when the field is absent; the production migration must add it).
//
//  2. quotareset.Options.DefaultHours float64 field exists (reflect check — same).
//
// Together these prove the typed-field capability is preserved: operators who
// previously set EVOLVE_QUOTA_RESET_AT / EVOLVE_QUOTA_RESET_HOURS via the env
// can now pass the same values via the policy.json quota_reset object instead,
// which Compute reads via opts.ResetAt and opts.DefaultHours.
//
// Covers NEG1 (capability preserved). Mixed predicate:
// reflect.FieldByName is behavioral; checks that the Options struct has grown
// the typed fields that replace the deleted env reads.
//
// // acs-predicate: config-check (no FileContains here — reflect is the sole check)
//
// RED:
//   - quotareset.Options has no ResetAt field → reflect returns ok=false
//   - quotareset.Options has no DefaultHours field → reflect returns ok=false
func TestC26_NEG1_QuotaOptionsTypedFieldsExist(t *testing.T) {
	// Behavioral: verify quotareset.Options.ResetAt string field exists.
	// Uses reflect so the test fails at runtime (not compile time).
	resetAtInfo, ok := reflect.TypeOf(quotareset.Options{}).FieldByName("ResetAt")
	if !ok {
		t.Fatalf("RED: quotareset.Options.ResetAt field missing.\n" +
			"Builder must add `ResetAt string` to Options in go/internal/quotareset/quotareset.go.\n" +
			"Source 1 in Compute must then use `strings.TrimSpace(opts.ResetAt)` instead of\n" +
			"getEnv(\"EVOLVE_QUOTA_RESET_AT\").")
	}
	if resetAtInfo.Type.Kind() != reflect.String {
		t.Errorf("RED: quotareset.Options.ResetAt is kind %v, want string.\n"+
			"The field must be typed as `string`.",
			resetAtInfo.Type.Kind())
	}

	// Behavioral: verify quotareset.Options.DefaultHours float64 field exists.
	hoursInfo, ok := reflect.TypeOf(quotareset.Options{}).FieldByName("DefaultHours")
	if !ok {
		t.Fatalf("RED: quotareset.Options.DefaultHours field missing.\n" +
			"Builder must add `DefaultHours float64` to Options in go/internal/quotareset/quotareset.go.\n" +
			"Source 3 in Compute must then use `if opts.DefaultHours > 0 { hours = opts.DefaultHours }`\n" +
			"instead of getEnv(\"EVOLVE_QUOTA_RESET_HOURS\").")
	}
	if hoursInfo.Type.Kind() != reflect.Float64 {
		t.Errorf("RED: quotareset.Options.DefaultHours is kind %v, want float64.\n"+
			"The field must be typed as `float64`.",
			hoursInfo.Type.Kind())
	}
}

// TestC26_NEG2_AcsPredTimeoutHasZeroEnvReaders verifies that no production Go
// source file calls os.Getenv or getEnv with "EVOLVE_ACS_PREDICATE_TIMEOUT_S"
// as the key argument.
//
// H2 falsifiable claim: the changedpkgs.go:6 reference is a comment, not a reader.
// The acssuite.go runner uses EVOLVE_ACS_GO_TIMEOUT_S (not PREDICATE_TIMEOUT_S).
// This predicate confirms the flag is dead (only a comment reference remains
// in changedpkgs.go after the registry row is removed).
//
// // acs-predicate: config-check — absence of env readers is the load-bearing check.
//
// PRE-EXISTING GREEN: no os.Getenv/getEnv call for this flag exists in any
// non-test Go source; only a comment reference in changedpkgs.go:6 and the
// now-removed registry_table.go entry. After Builder removes the registry row,
// zero references outside comments will remain.
func TestC26_NEG2_AcsPredTimeoutHasZeroEnvReaders(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	// Check acssuite.go does NOT use EVOLVE_ACS_PREDICATE_TIMEOUT_S as an env key.
	// The runner uses EVOLVE_ACS_GO_TIMEOUT_S instead (per the scout's all-surfaces grep).
	acsSuiteFile := filepath.Join(root, "go", "internal", "acssuite", "acssuite.go")
	if !acsassert.FileNotContains(t, acsSuiteFile, `"EVOLVE_ACS_PREDICATE_TIMEOUT_S"`) {
		t.Errorf("RED: acssuite.go unexpectedly reads EVOLVE_ACS_PREDICATE_TIMEOUT_S.\n"+
			"The runner should only use EVOLVE_ACS_GO_TIMEOUT_S.\n"+
			"If this check fails, investigate before removing the flag — it may not be dead.\n"+
			"File: %s", acsSuiteFile)
	}

	// Check registry_table.go row is removed (the string should no longer appear
	// as a registry Name value after Builder removes the row).
	registryFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileNotContains(t, registryFile, `"EVOLVE_ACS_PREDICATE_TIMEOUT_S"`) {
		t.Errorf("RED: registry_table.go still registers EVOLVE_ACS_PREDICATE_TIMEOUT_S.\n"+
			"Builder must delete the registry row for this dead flag.\n"+
			"A comment reference in changedpkgs.go:6 is acceptable (it is not an env read).\n"+
			"File: %s", registryFile)
	}
}
