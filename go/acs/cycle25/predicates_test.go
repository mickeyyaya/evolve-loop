//go:build acs

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

var removedFlags = []string{
	"EVOLVE_INTERACTIVE_POLICY",
	"EVOLVE_SCOUT_INTERACTIVE_POLICY",
	"EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY",
}

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

func TestC25_004_NoProductionReaderForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

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

func TestC25_005_BridgeRequestInteractivePolicyTypedField(t *testing.T) {
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

func TestC25_NEG1_ProfileInteractivePolicyHonored(t *testing.T) {
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

func TestC25_NEG2_RuntimeReferenceStillDocumentsInteractivePolicy(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	runtimeRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.FileContains(t, runtimeRef, "EVOLVE_INTERACTIVE_POLICY") {
		t.Errorf("RED: runtime-reference.md no longer documents EVOLVE_INTERACTIVE_POLICY.\n"+
			"Builder must NOT delete the operator-facing runtime-reference.md row for\n"+
			"EVOLVE_INTERACTIVE_POLICY during the bridge.go / registry migration.\n"+
			"The flag is removed from the registry (os.Getenv deleted) but remains a\n"+
			"valid reqEnv config surface documented for operators.\n"+
			"File: %s", runtimeRef)
	}
}
