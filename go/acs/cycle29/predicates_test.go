//go:build acs

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
package cycle29

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// workflowFlags is the canonical list of 5 flags that cycle-29 removes by
// migrating them into policy.WorkflowConfig (Configuration Object pattern):
//   - EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS: resolved to WorkflowConfig.MaxConsecutiveFails
//   - EVOLVE_MAX_CYCLES_CAP:             resolved to WorkflowConfig.MaxCyclesCap
//   - EVOLVE_AUTO_PRUNE:                 resolved to WorkflowConfig.AutoPrune
//   - EVOLVE_DIFF_COMPLEXITY_DISABLE:    resolved to WorkflowConfig.DiffComplexityDisable
//   - EVOLVE_AUDITOR_TIER_OVERRIDE:      resolved to WorkflowConfig.AuditorTierOverride
var workflowFlags = []string{
	"EVOLVE_AUDITOR_TIER_OVERRIDE",
	"EVOLVE_AUTO_PRUNE",
	"EVOLVE_DIFF_COMPLEXITY_DISABLE",
	"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS",
	"EVOLVE_MAX_CYCLES_CAP",
}

// TestC29_001_WorkflowFlagsAbsentFromRegistry verifies that all 5 workflow-defaults
// flags are no longer registered after Builder removes their rows from registry_table.go.
//
// Covers AC1. All 5 flags migrate to policy.WorkflowConfig (bucket 1 —
// Configuration Object), so their registry rows are deleted.
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 5 flags are currently registered (FlagCeiling=120); each Lookup
// returns (flag, true).
func TestC29_001_WorkflowFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range workflowFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-29 workflow-config-cluster-29).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC29_004_NoEnvReadsForRemovedFlags verifies that the os.Getenv and envchain
// read sites for the 5 workflow-defaults flags have been deleted from their
// respective source files.
//
// Covers AC4. Config-check waiver: FileNotContains asserts structural absence
// of the exact quoted env-key strings. The 5 literals span 3 files:
//   - cmd_loop.go:          EVOLVE_AUTO_PRUNE, EVOLVE_MAX_CYCLES_CAP (inline in resolveMaxCyclesCap)
//   - cmd_loop_control.go:  EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS (in resolveMaxConsecutiveFails)
//   - cmd_subagent.go:      EVOLVE_AUDITOR_TIER_OVERRIDE × 2, EVOLVE_DIFF_COMPLEXITY_DISABLE × 2
//
// acs-predicate: config-check
//
// RED:
//
//	cmd_loop.go:122         has `os.Getenv("EVOLVE_AUTO_PRUNE")`
//	cmd_loop.go:548         has `os.Getenv("EVOLVE_MAX_CYCLES_CAP")`
//	cmd_loop_control.go:123 has `os.Getenv("EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS")`
//	cmd_subagent.go:192     has `os.Getenv("EVOLVE_AUDITOR_TIER_OVERRIDE")`
//	cmd_subagent.go:193     has `envchain.Bool("EVOLVE_DIFF_COMPLEXITY_DISABLE", nil, false)`
//	cmd_subagent.go:359     has `os.Getenv("EVOLVE_AUDITOR_TIER_OVERRIDE")`
//	cmd_subagent.go:360     has `envchain.Bool("EVOLVE_DIFF_COMPLEXITY_DISABLE", nil, false)`
func TestC29_004_NoEnvReadsForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file  string
		flags []string
	}{
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go"),
			[]string{`"EVOLVE_AUTO_PRUNE"`, `"EVOLVE_MAX_CYCLES_CAP"`},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_control.go"),
			[]string{`"EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS"`},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_subagent.go"),
			[]string{`"EVOLVE_AUDITOR_TIER_OVERRIDE"`, `"EVOLVE_DIFF_COMPLEXITY_DISABLE"`},
		},
	}
	for _, c := range checks {
		for _, literal := range c.flags {
			if !acsassert.FileNotContains(t, c.file, literal) {
				t.Errorf("RED: %s still contains env-read literal %q.\n"+
					"Builder must delete the os.Getenv/envchain call for this flag and replace\n"+
					"it with the appropriate WorkflowConfig field via pol.WorkflowConfig().\n"+
					"File: %s", filepath.Base(c.file), literal, c.file)
			}
		}
	}
}

// TestC29_005_WorkflowConfigStructExistsInPolicy verifies that:
//
//  1. policy.Policy has a Workflow field (reflect check — fails at runtime when
//     the field is absent, giving a precise per-test failure message without
//     a whole-file compile error).
//  2. The Workflow field is a pointer type (*WorkflowPolicy).
//  3. The pointed-to struct has MaxConsecutiveFails int, MaxCyclesCap int,
//     AutoPrune *bool, DiffComplexityDisable bool, and AuditorTierOverride string
//     sub-fields — the typed replacements for the deleted env reads.
//  4. WorkflowConfig() resolver returns the correct defaults when Policy.Workflow == nil:
//     MaxConsecutiveFails=1, MaxCyclesCap=25, AutoPrune=true.
//
// Covers AC5. BEHAVIORAL: reflect.FieldByName traverses the production type
// system and a direct resolver call returns computed values; a magic-string
// source edit cannot satisfy this — the struct and fields must actually exist
// for reflect to find them and the resolver must compute the right defaults.
//
// RED:
//
//	policy.Policy has no Workflow field → reflect returns ok=false at the first check.
//	(post-field-add) WorkflowPolicy lacks required fields → inner checks fail.
//	(post-struct-add) WorkflowConfig() defaults don't match spec → final checks fail.
func TestC29_005_WorkflowConfigStructExistsInPolicy(t *testing.T) {
	// Check Policy has Workflow field.
	pInfo, ok := reflect.TypeOf(policy.Policy{}).FieldByName("Workflow")
	if !ok {
		t.Fatalf("RED: policy.Policy.Workflow field missing.\n" +
			"Builder must add `Workflow *WorkflowPolicy` to Policy in go/internal/policy/policy.go\n" +
			"(parallel to Dispatch *DispatchConfig from cycle-28).")
	}
	if pInfo.Type.Kind() != reflect.Ptr {
		t.Fatalf("RED: policy.Policy.Workflow is kind %v, want pointer (*WorkflowPolicy).\n"+
			"The field must be typed as `*WorkflowPolicy`.",
			pInfo.Type.Kind())
	}

	// Navigate to the WorkflowPolicy struct type via the pointer's element type.
	wfType := pInfo.Type.Elem()
	requiredIntFields := []string{"MaxConsecutiveFails", "MaxCyclesCap"}
	for _, fname := range requiredIntFields {
		if f, ok := wfType.FieldByName(fname); ok {
			if f.Type.Kind() != reflect.Int {
				t.Errorf("RED: WorkflowPolicy.%s is kind %v, want int.", fname, f.Type.Kind())
			}
		} else {
			t.Errorf("RED: WorkflowPolicy missing %s field.\n"+
				"Builder must add `%s int` to WorkflowPolicy.", fname, fname)
		}
	}
	if f, ok := wfType.FieldByName("AutoPrune"); ok {
		if f.Type.Kind() != reflect.Ptr {
			t.Errorf("RED: WorkflowPolicy.AutoPrune is kind %v, want *bool (nil = default true).",
				f.Type.Kind())
		}
	} else {
		t.Errorf("RED: WorkflowPolicy missing AutoPrune field.\n" +
			"Builder must add `AutoPrune *bool` to WorkflowPolicy (nil = default true).")
	}
	if f, ok := wfType.FieldByName("DiffComplexityDisable"); ok {
		if f.Type.Kind() != reflect.Bool {
			t.Errorf("RED: WorkflowPolicy.DiffComplexityDisable is kind %v, want bool.",
				f.Type.Kind())
		}
	} else {
		t.Errorf("RED: WorkflowPolicy missing DiffComplexityDisable field.\n" +
			"Builder must add `DiffComplexityDisable bool` to WorkflowPolicy.")
	}
	if f, ok := wfType.FieldByName("AuditorTierOverride"); ok {
		if f.Type.Kind() != reflect.String {
			t.Errorf("RED: WorkflowPolicy.AuditorTierOverride is kind %v, want string.",
				f.Type.Kind())
		}
	} else {
		t.Errorf("RED: WorkflowPolicy missing AuditorTierOverride field.\n" +
			"Builder must add `AuditorTierOverride string` to WorkflowPolicy.")
	}

	// Verify WorkflowConfig() resolver returns correct defaults when Workflow == nil.
	wc := policy.Policy{}.WorkflowConfig()
	if wc.MaxConsecutiveFails != 1 {
		t.Errorf("RED: WorkflowConfig().MaxConsecutiveFails = %d, want 1 (matches legacy EVOLVE_LOOP_MAX_CONSECUTIVE_FAILS default).",
			wc.MaxConsecutiveFails)
	}
	if wc.MaxCyclesCap != 25 {
		t.Errorf("RED: WorkflowConfig().MaxCyclesCap = %d, want 25 (matches legacy EVOLVE_MAX_CYCLES_CAP default).",
			wc.MaxCyclesCap)
	}
	if !wc.AutoPrune {
		t.Errorf("RED: WorkflowConfig().AutoPrune = false, want true (matches legacy EVOLVE_AUTO_PRUNE != '0' default).")
	}
}

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
func TestC29_007_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// TestC29_E01_WorkflowConfigEmptyPolicyDefaults verifies that an explicit but
// empty WorkflowPolicy{} block in policy.json (Policy.Workflow = &WorkflowPolicy{})
// results in the same correct defaults as a nil Workflow pointer.
//
// Covers E01 (edge/OOD: zero-value struct). This tests the resolver's
// zero-value handling: int 0 → use default, nil *bool → use default true,
// empty string → use default "".
//
// BEHAVIORAL: constructs a real policy.Policy value and calls WorkflowConfig()
// directly — no source grepping. The resolver must distinguish "zero int" from
// "user provided 0" correctly for MaxConsecutiveFails and MaxCyclesCap.
//
// RED: WorkflowConfig() method does not yet exist; calling it fails to compile.
func TestC29_E01_WorkflowConfigEmptyPolicyDefaults(t *testing.T) {
	wc := policy.Policy{Workflow: &policy.WorkflowPolicy{}}.WorkflowConfig()
	if wc.MaxConsecutiveFails != 1 {
		t.Errorf("E01: empty WorkflowPolicy{}.MaxConsecutiveFails resolved to %d, want 1.\n"+
			"A zero int must fall through to the default (1), not override it.",
			wc.MaxConsecutiveFails)
	}
	if wc.MaxCyclesCap != 25 {
		t.Errorf("E01: empty WorkflowPolicy{}.MaxCyclesCap resolved to %d, want 25.\n"+
			"A zero int must fall through to the default (25), not override it.",
			wc.MaxCyclesCap)
	}
	if !wc.AutoPrune {
		t.Errorf("E01: empty WorkflowPolicy{}.AutoPrune resolved to false, want true.\n" +
			"A nil *bool must fall through to the default (true).")
	}
	if wc.DiffComplexityDisable {
		t.Errorf("E01: empty WorkflowPolicy{}.DiffComplexityDisable resolved to true, want false.\n" +
			"A zero bool must remain false (disable=false = complexity check enabled by default).")
	}
	if wc.AuditorTierOverride != "" {
		t.Errorf("E01: empty WorkflowPolicy{}.AuditorTierOverride resolved to %q, want \"\".\n"+
			"An empty string must remain empty (no tier override by default).",
			wc.AuditorTierOverride)
	}
}
