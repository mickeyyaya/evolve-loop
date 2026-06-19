//go:build acs

// Package cycle25 materializes the cycle-25 acceptance criteria for:
//
//	interactive-policy-profiles — remove 3 EVOLVE_INTERACTIVE_POLICY cluster flags
//	(EVOLVE_INTERACTIVE_POLICY, EVOLVE_SCOUT_INTERACTIVE_POLICY,
//	EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY) by deleting os.Getenv tier-2 reads
//	(envchain.Resolve in bridge.go::resolvePolicy) and adding Profile SSOT
//	(Profile.InteractivePolicy field) + typed BridgeRequest.InteractivePolicy field.
//	Lower FlagCeiling 135→132, regenerate docs/architecture/control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	interactive-policy-profiles:
//	  AC1  3 flags absent from Lookup             → C25_001 (behavioral)
//	  AC2  Registry row count == 132              → C25_002 (behavioral, count)
//	  AC3  FlagCeiling const == 132               → C25_003 (config-check, waiver)
//	  AC4  No prod readers + docs_contract pruned → C25_004 (mixed, config-check waiver)
//	  AC5  BridgeRequest.InteractivePolicy field  → C25_005 (behavioral, reflect)
//	  AC6  flagreaders guard green                → manual+checklist (see below)
//	  AC7  WORKTREE_PATH still registered         → C25_007 (behavioral — PRE-EXISTING GREEN)
//	  AC8  control-flags.md has no removed rows   → C25_008 (config-check, waiver)
//	  NEG1 Profile.InteractivePolicy honored      → C25_NEG1 (behavioral, reflect)
//	  NEG2 runtime-reference.md still docs flag   → C25_NEG2 (config-check, waiver — PRE-EXISTING GREEN)
//
// ACs with manual+checklist disposition:
//
//	AC6 (flagreaders guard green): `go test -tags acs ./acs/regression/flagreaders/...`
//	    Checklist for Auditor:
//	    (a) no compile errors with -tags acs on the cycle25 package;
//	    (b) exit 0 from `go test -tags acs ./acs/regression/flagreaders/...`;
//	    (c) no stale EVOLVE_INTERACTIVE_POLICY / EVOLVE_SCOUT_INTERACTIVE_POLICY /
//	        EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY literal strings remain in
//	        non-test production Go (grep -rn 'EVOLVE_INTERACTIVE_POLICY\|...' go/ --include='*.go'
//	        | grep -v '_test.go' | grep -v 'registry_table.go' → 0 matches).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C25_005 — uses reflect to verify BridgeRequest.InteractivePolicy field
//	           exists AND verifies bridge.go passes req.InteractivePolicy to resolvePolicy.
//	           The field-level behavioral check (reflect.FieldByName) is the strongest
//	           anti-no-op signal: adding a magic comment or grep string cannot satisfy it
//	           — the struct field must exist.
//	           C25_NEG1 — uses reflect to verify Profile.InteractivePolicy field exists
//	           AND checks runner.go reads prof.InteractivePolicy. Both checks are required:
//	           field existence alone does not prove the runner wires it to BridgeRequest.
//	Edge/OOD:  C25_001 tests ALL 3 flags in the cluster; includes EVOLVE_INTERACTIVE_POLICY
//	           (global) and 2 per-agent variants, ensuring no partial removal.
//	           C25_004 covers both the resolvePolicy envchain.Resolve call site (2 calls)
//	           AND the docs_contract_test.go cleanup (5 dead allowedUndocumented entries).
//	Lexical:   Lookup / len / FileContains / FileNotContains / CountInGoFunc /
//	           reflect.FieldByName / reflect.Kind — seven distinct verbs.
//	Semantic:  registry-absence, row-count, ceiling-const, structural-reader-absence,
//	           typed-field-reflection, doc-absence, worktree-path-preserved,
//	           profile-field-reflection, runtime-reference-preserved — 9 distinct behaviors.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (interactive-policy-profiles). Deferred tasks (WORKTREE_PATH, ROUTER_CLI/MODEL,
// Workflow Defaults, BYPASS_* cluster) get zero predicates.
//
// 1:1 enforcement: predicate=9, manual+checklist=1, unverifiable-remove=0 → total AC=10 ✓
package cycle25

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// removedFlags is the canonical list of 3 INTERACTIVE_POLICY cluster flags
// that cycle-25 removes by migrating from envchain.Resolve (os.Getenv tier-2)
// to Profile SSOT + typed BridgeRequest.InteractivePolicy field.
var removedFlags = []string{
	"EVOLVE_INTERACTIVE_POLICY",
	"EVOLVE_SCOUT_INTERACTIVE_POLICY",
	"EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY",
}

// TestC25_001_DeadFlagsAbsentFromRegistry verifies that all 3 INTERACTIVE_POLICY
// cluster flags are no longer registered after Builder removes their rows from
// registry_table.go.
//
// Covers AC1. The 3 flags span two lookup patterns:
//   - EVOLVE_INTERACTIVE_POLICY: global policy key, bridge.go:195 envchain.Resolve
//   - EVOLVE_SCOUT_INTERACTIVE_POLICY: per-agent, bridge.go:191 dynamic dispatch
//   - EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY: per-agent, same dispatch
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the registry row must be absent for
// Lookup to return ok=false.
//
// RED: all 3 flags are currently registered; each Lookup returns (flag, true).
func TestC25_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-25 interactive-policy-profiles).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC25_002_RegistryRowCountIs132 verifies that after removing all 3 rows the
// total registry count is exactly 132.
//
// Covers AC2. Both over-removal (< 132) and under-removal (> 132) fail.
//
// BEHAVIORAL: calls len(flagregistry.All) directly (the production SSOT slice).
// No source-file grepping; a magic-string patch cannot satisfy this.
//
// RED: len(flagregistry.All) is currently 135, which is 3 rows above 132.
func TestC25_002_RegistryRowCountIs132(t *testing.T) {
	const want = 132
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 3 INTERACTIVE_POLICY cluster rows from registry_table.go.\n"+
			"Both over-removal (< 132) and under-removal (> 132) fail.\n"+
			"Expected: 135 − 3 = 132.",
			got, want)
	}
}

// TestC25_003_FlagCeilingConstIs132 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 135 to 132
// in the same diff as the 3-row removal.
//
// // acs-predicate: config-check — the constant value is the canonical ratchet;
// keeping 135 after the 3-row removal breaks the ratchet guarantee.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 135.
func TestC25_003_FlagCeilingConstIs132(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 132") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 132'.\n"+
			"Builder must lower the FlagCeiling constant from 135 to 132 in the same diff\n"+
			"as removing the 3 INTERACTIVE_POLICY cluster rows (135 − 3 = 132).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC25_004_NoProductionReaderForRemovedFlags verifies that the os.Getenv
// tier-2 read sites for the 3 removed flags have been deleted:
//
//  1. bridge.go::resolvePolicy no longer calls envchain.Resolve at all.
//     (The function had 2 calls: one for per-agent key, one for EVOLVE_INTERACTIVE_POLICY.)
//
//  2. docs_contract_test.go allowedUndocumented no longer has the 5 dead
//     per-agent INTERACTIVE_POLICY entries (SCOUT/BUILDER/AUDITOR/TDD_ENGINEER/PLAN_REVIEWER).
//     After migration these are covered by the built-in FAMILY pattern exemption in
//     the test's own comment block; explicit entries are dead technical debt.
//
// Covers AC4. Mixed predicate: CountInGoFunc is structural (config-check waiver);
// FileNotContains on docs_contract_test.go is also structural (config-check waiver).
//
// // acs-predicate: config-check
//
// RED:
//   - resolvePolicy has 2 envchain.Resolve calls (per-agent + global)
//   - docs_contract_test.go has 5 INTERACTIVE_POLICY entries at lines 62-66
func TestC25_004_NoProductionReaderForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	// Check 1: bridge.go resolvePolicy has 0 envchain.Resolve calls.
	// Currently: 2 calls (envchain.Resolve(perAgentPolicyEnv(agent), ...) at line 191
	// and envchain.Resolve("EVOLVE_INTERACTIVE_POLICY", ...) at line 195).
	// After fix: reqEnv map lookup only; no os.Getenv tier.
	bridgeFile := filepath.Join(root, "go", "internal", "adapters", "bridge", "bridge.go")
	count, err := acsassert.CountInGoFunc(bridgeFile, "resolvePolicy", "envchain.Resolve")
	if err != nil {
		t.Fatalf("CountInGoFunc(resolvePolicy, envchain.Resolve): %v", err)
	}
	if count > 0 {
		t.Errorf("RED: resolvePolicy in bridge.go still calls envchain.Resolve %d time(s).\n"+
			"Builder must replace both envchain.Resolve calls with reqEnv map lookups\n"+
			"and add a profilePolicy parameter for the Profile tier.\n"+
			"File: %s", count, bridgeFile)
	}

	// Check 2: docs_contract_test.go no longer has the 5 dead allowedUndocumented
	// INTERACTIVE_POLICY entries. After migration the per-agent variants are covered
	// by the built-in FAMILY pattern exemption (see the "allow EVOLVE_<PHASE>_*"
	// comment block in docs_contract_test.go). Explicit entries are dead debt.
	docsContractFile := filepath.Join(root, "go", "cmd", "evolve", "docs_contract_test.go")
	for _, dead := range []string{
		"EVOLVE_SCOUT_INTERACTIVE_POLICY",
		"EVOLVE_BUILDER_INTERACTIVE_POLICY",
		"EVOLVE_AUDITOR_INTERACTIVE_POLICY",
		"EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY",
		"EVOLVE_PLAN_REVIEWER_INTERACTIVE_POLICY",
	} {
		if !acsassert.FileNotContains(t, docsContractFile, dead) {
			t.Errorf("RED: docs_contract_test.go still has %q in allowedUndocumented.\n"+
				"Builder must prune this dead entry — the per-phase INTERACTIVE_POLICY variants\n"+
				"are already covered by the built-in FAMILY pattern exemption.\n"+
				"File: %s", dead, docsContractFile)
		}
	}
}

// TestC25_005_BridgeRequestInteractivePolicyTypedField verifies that:
//
//  1. BridgeRequest.InteractivePolicy string field exists (reflect check — fails at
//     runtime when field is absent, avoiding a whole-file compile error).
//
//  2. bridge.go calls resolvePolicy with req.InteractivePolicy (the DI wiring that
//     connects BridgeRequest → resolvePolicy, closing the os.Getenv bypass).
//
// Covers AC5 (typed field + sentinel suppression). Mixed predicate:
// reflect.FieldByName is behavioral; FileContains on bridge.go is structural
// (config-check waiver for the call-site wiring).
//
// // acs-predicate: config-check (FileContains portion)
//
// RED:
//   - BridgeRequest has no InteractivePolicy field in ports.go → reflect returns ok=false
//   - bridge.go call site at line ~140 does not pass req.InteractivePolicy to resolvePolicy
func TestC25_005_BridgeRequestInteractivePolicyTypedField(t *testing.T) {
	// Behavioral: verify BridgeRequest.InteractivePolicy string field exists.
	// Uses reflect.FieldByName so the test fails at runtime (not compile time)
	// when the field is absent, giving a meaningful per-test failure message.
	fInfo, ok := reflect.TypeOf(core.BridgeRequest{}).FieldByName("InteractivePolicy")
	if !ok {
		t.Fatalf("RED: core.BridgeRequest.InteractivePolicy field missing.\n" +
			"Builder must add `InteractivePolicy string` to BridgeRequest in go/internal/core/ports.go\n" +
			"(parallel to PermissionMode, added in cycle-24).")
	}
	if fInfo.Type.Kind() != reflect.String {
		t.Errorf("RED: BridgeRequest.InteractivePolicy is kind %v, want string.\n"+
			"The field must be typed as `string`, not %v.",
			fInfo.Type.Kind(), fInfo.Type.Kind())
	}

	// Structural: bridge.go's resolvePolicy call site passes req.InteractivePolicy.
	// After fix: resolvePolicy(req.Agent, req.Env, req.InteractivePolicy)
	// Before fix: resolvePolicy(req.Agent, req.Env) — no profilePolicy argument.
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	bridgeFile := filepath.Join(root, "go", "internal", "adapters", "bridge", "bridge.go")
	if !acsassert.FileContains(t, bridgeFile, "req.InteractivePolicy") {
		t.Errorf("RED: bridge.go does not pass req.InteractivePolicy to resolvePolicy.\n"+
			"Builder must update the call site at bridge.go:~140 from\n"+
			"  resolvePolicy(req.Agent, req.Env)\n"+
			"to\n"+
			"  resolvePolicy(req.Agent, req.Env, req.InteractivePolicy)\n"+
			"File: %s", bridgeFile)
	}
}

// TestC25_007_WorktreePathStillInRegistry verifies that EVOLVE_WORKTREE_PATH
// remains in the registry after the 3-row removal — it is a live IPC handoff
// (agents/evolve-tester.md) pinned by TestC50_009.
//
// Covers AC7 (WORKTREE_PATH must not be touched). Cycles 17, 18, and 19 all
// failed when Builder over-reached and removed WORKTREE_PATH, breaking TestC50_009.
//
// BEHAVIORAL: calls flagregistry.Lookup("EVOLVE_WORKTREE_PATH") — the production SSOT.
//
// PRE-EXISTING GREEN: WORKTREE_PATH is currently registered and must stay so.
func TestC25_007_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by TestC50_009.\n"+
			"This is the same mistake that killed cycles 17, 18, and 19.",
			worktreePath)
	}
}

// TestC25_008_ControlFlagsMdHasNoRemovedRows verifies that the generated doc
// docs/architecture/control-flags.md has no entries for any of the 3 removed flags
// after the registry rows are removed and the doc regenerated via 'evolve flags generate'.
//
// Covers AC8. The doc is generated from the flagregistry (source of truth);
// absence follows from C25_001 (rows removed) plus regeneration.
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has entries for all 3 removed flags.
func TestC25_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, flag := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 3 INTERACTIVE_POLICY rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

// TestC25_NEG1_ProfileInteractivePolicyHonored verifies that:
//
//  1. profiles.Profile.InteractivePolicy string field exists (reflect check).
//
//  2. runner.go reads prof.InteractivePolicy and passes it through to BridgeRequest,
//     proving the capability is preserved (Profile tier-3 is operative after
//     the os.Getenv tier-2 removal).
//
// Covers NEG1 (capability preservation). Mixed predicate:
// reflect.FieldByName is behavioral; FileContains on runner.go is structural
// (config-check waiver for the resolution wiring).
//
// // acs-predicate: config-check (FileContains portion)
//
// RED:
//   - Profile has no InteractivePolicy field in profiles.go → reflect returns ok=false
//   - runner.go does not read prof.InteractivePolicy (not yet wired)
func TestC25_NEG1_ProfileInteractivePolicyHonored(t *testing.T) {
	// Behavioral: verify Profile.InteractivePolicy string field exists.
	// Uses reflect so the test fails at runtime (not compile time).
	fInfo, ok := reflect.TypeOf(profiles.Profile{}).FieldByName("InteractivePolicy")
	if !ok {
		t.Fatalf("RED: profiles.Profile.InteractivePolicy field missing.\n" +
			"Builder must add `InteractivePolicy string` to Profile in go/internal/profiles/profiles.go\n" +
			"(parallel to PermissionMode, added in cycle-24 as a Profile field).")
	}
	if fInfo.Type.Kind() != reflect.String {
		t.Errorf("RED: Profile.InteractivePolicy is kind %v, want string.\n"+
			"The field must be typed as `string`, not %v.",
			fInfo.Type.Kind(), fInfo.Type.Kind())
	}

	// Structural: runner.go resolution block reads prof.InteractivePolicy.
	// After fix: interactivePolicy := req.Env[perAgentKey]; if == "" && prof != nil {
	//                interactivePolicy = prof.InteractivePolicy }
	// Before fix: the field does not exist; runner.go has no such read.
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	runnerFile := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileContains(t, runnerFile, "prof.InteractivePolicy") {
		t.Errorf("RED: runner.go does not read prof.InteractivePolicy.\n"+
			"Builder must add a resolution block (parallel to PermissionMode at runner.go:~445):\n"+
			"  interactivePolicy := req.Env[envchain.PhaseEnvKey(profileName, \"INTERACTIVE_POLICY\")]\n"+
			"  if interactivePolicy == \"\" && prof != nil {\n"+
			"      interactivePolicy = prof.InteractivePolicy\n"+
			"  }\n"+
			"  if interactivePolicy == \"\" {\n"+
			"      interactivePolicy = req.Env[\"EVOLVE_INTERACTIVE_POLICY\"]\n"+
			"  }\n"+
			"Then pass InteractivePolicy: interactivePolicy in the BridgeRequest literal.\n"+
			"File: %s", runnerFile)
	}
}

// TestC25_NEG2_RuntimeReferenceStillDocumentsInteractivePolicy verifies that
// docs/operations/runtime-reference.md retains the EVOLVE_INTERACTIVE_POLICY row
// after the registry removal and bridge.go migration.
//
// Covers NEG2 (doc preservation). Even though the flag is removed from the registry
// (os.Getenv reader deleted), EVOLVE_INTERACTIVE_POLICY is still a valid operator
// config surface (reqEnv key) and its runtime-reference.md documentation should NOT
// be deleted as collateral cleanup. The migration changes HOW the value is sourced
// (reqEnv → profile → reqEnv global instead of os.Getenv), not whether it exists.
//
// // acs-predicate: config-check — reference doc must retain this operator-facing entry.
//
// PRE-EXISTING GREEN: runtime-reference.md currently documents EVOLVE_INTERACTIVE_POLICY
// (at line ~94). This predicate guards against accidental deletion during migration.
func TestC25_NEG2_RuntimeReferenceStillDocumentsInteractivePolicy(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	runtimeRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	// The global flag (EVOLVE_INTERACTIVE_POLICY) must remain in runtime-reference.md
	// because it is still a valid reqEnv operator config key after migration.
	// Per-agent variants (SCOUT, TDD_ENGINEER) were StatusInternal and are
	// intentionally absent from runtime-reference.md (never operator dials).
	if !acsassert.FileContains(t, runtimeRef, "EVOLVE_INTERACTIVE_POLICY") {
		t.Errorf("RED: runtime-reference.md no longer documents EVOLVE_INTERACTIVE_POLICY.\n"+
			"Builder must NOT delete the operator-facing runtime-reference.md row for\n"+
			"EVOLVE_INTERACTIVE_POLICY during the bridge.go / registry migration.\n"+
			"The flag is removed from the registry (os.Getenv deleted) but remains a\n"+
			"valid reqEnv config surface documented for operators.\n"+
			"File: %s", runtimeRef)
	}
}
