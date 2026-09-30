//go:build acs

package cycle39

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var legacyRemovedFlags = []string{
	"EVOLVE_REQUIRE_INTENT",
	"EVOLVE_TRIAGE_DISABLE",
	"EVOLVE_PLAN_REVIEW",
	"EVOLVE_TEST_PHASE_ENABLED",
	"EVOLVE_BUILD_PLANNER",
	"EVOLVE_SWARM_PLANNER",
}

func TestC39_001_LegacyFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range legacyRemovedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go\n"+
				"(legacyflags-phase-enable-cluster-39: migrate to WorkflowPolicy.PhaseEnables).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC39_004_NoProdLegacyFlagEnvReadsInSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	configFile := filepath.Join(root, "go", "internal", "config", "config.go")
	cyclerunFile := filepath.Join(root, "go", "internal", "core", "cyclerun.go")

	for _, name := range legacyRemovedFlags {
		if !acsassert.FileNotContains(t, configFile, name) {
			t.Errorf("RED: config.go still contains the string %q.\n"+
				"Builder must delete the legacyFlags map entry for %q AND the for-loop\n"+
				"iteration at lines ~557–568 that reads env[flag] (cycle-8 anti-gaming:\n"+
				"removing the registry row without deleting the env read is split-const hiding).\n"+
				"File: %s", name, name, configFile)
		}
	}

	const directRead = "EVOLVE_REQUIRE_INTENT"
	if !acsassert.FileNotContains(t, cyclerunFile, directRead) {
		t.Errorf("RED: cyclerun.go still contains the string %q.\n"+
			"Builder must replace envchain.BoolValue(req.Env[\"EVOLVE_REQUIRE_INTENT\"], false)\n"+
			"with o.workflowConfig.PhaseEnables[\"intent\"] == \"on\" in newCycleRun (line ~240).\n"+
			"File: %s", directRead, cyclerunFile)
	}
}

func TestC39_005_LegacyFlagsVarDeletedFromConfig(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	configFile := filepath.Join(root, "go", "internal", "config", "config.go")

	if !acsassert.FileNotContains(t, configFile, "legacyFlags = map[string]legacyFlag{") {
		t.Errorf("RED: config.go still contains the legacyFlags map declaration.\n"+
			"Builder must delete the entire `var legacyFlags = map[string]legacyFlag{...}`\n"+
			"block (lines 306–324) from config.go. An empty map would still compile but\n"+
			"leaves dead code; the map and its for-loop must both be deleted.\n"+
			"File: %s", configFile)
	}

	if !acsassert.FileNotContains(t, configFile, "for flag, lf := range legacyFlags") {
		t.Errorf("RED: config.go still contains the legacyFlags for-loop.\n"+
			"Builder must delete the `for flag, lf := range legacyFlags { ... }` block\n"+
			"(lines ~557–568) from config.go in the same diff as deleting the map var.\n"+
			"File: %s", configFile)
	}
}

func TestC39_006_WorkflowPolicyPhaseEnablesResolves(t *testing.T) {
	cfg := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			PhaseEnables: map[string]string{"intent": "on"},
		},
	}.WorkflowConfig()

	if cfg.PhaseEnables["intent"] != "on" {
		t.Errorf("RED: WorkflowConfig().PhaseEnables[%q] = %q, want \"on\".\n"+
			"Builder must add PhaseEnables map[string]string to WorkflowPolicy AND\n"+
			"WorkflowConfig, then wire it in WorkflowConfig() resolver:\n"+
			"  c.PhaseEnables = p.Workflow.PhaseEnables",
			"intent", cfg.PhaseEnables["intent"])
	}

	dflt := policy.Policy{}.WorkflowConfig()
	if len(dflt.PhaseEnables) != 0 {
		t.Errorf("RED: policy.Policy{}.WorkflowConfig().PhaseEnables has %d entries, want 0.\n"+
			"An absent/empty Workflow block must not override phase enables.\n"+
			"When Workflow is nil, c.PhaseEnables must be nil (Go zero value of map).",
			len(dflt.PhaseEnables))
	}
}

func TestC39_007_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

func TestC39_009_ControlFlagsDocNoLegacyFlagRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range legacyRemovedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"the 6 legacy flag rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

// acs-predicate: config-check
func TestC39_NEG1_IntentRequiredUsesPhaseEnabledNotEnvInjection(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	bricksFile := filepath.Join(root, "go", "internal", "routingtest", "bricks.go")

	if !acsassert.FileNotContains(t, bricksFile, `"EVOLVE_REQUIRE_INTENT"`) {
		t.Errorf("RED: bricks.go still contains the string literal \"EVOLVE_REQUIRE_INTENT\".\n"+
			"Builder must change IntentRequired() from:\n"+
			"  s.Env[\"EVOLVE_REQUIRE_INTENT\"] = \"1\"  (env injection, now deleted code path)\n"+
			"to:\n"+
			"  s.Enable[\"intent\"] = config.EnableOn  (PhaseEnabled pattern; engine uses s.Enable directly)\n"+
			"Routing engine uses PhaseEnable: s.Enable (engine.go:52); env injection was already\n"+
			"disconnected from routing behavior — this is a correctness fix, not just cleanup.\n"+
			"File: %s", bricksFile)
	}
}

func TestC39_CA_001_ConsensusAuditAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CONSENSUS_AUDIT"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove the EVOLVE_CONSENSUS_AUDIT row from registry_table.go\n"+
			"(consensus-audit-config-39: migrate to WorkflowPolicy.ConsensusAuditEnabled).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_CONSENSUS_AUDIT", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC39_CA_004_NoProdConsensusAuditEnvReads(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	dispatchFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_consensus_dispatch.go")
	literal := `"EVOLVE_CONSENSUS_AUDIT"`
	if !acsassert.FileNotContains(t, dispatchFile, literal) {
		t.Errorf("RED: cmd_consensus_dispatch.go still contains the string literal %s.\n"+
			"Builder must replace os.Getenv(\"EVOLVE_CONSENSUS_AUDIT\") == \"0\" with:\n"+
			"  pol, _ := policy.Load(filepath.Join(projectRoot, \".evolve\", \"policy.json\"))\n"+
			"  ConsensusEnvOff: !pol.WorkflowConfig().ConsensusAuditEnabled\n"+
			"(cycle-8 anti-gaming: removing the registry row without deleting the os.Getenv\n"+
			"call is the split-const hiding pattern).\n"+
			"File: %s", literal, dispatchFile)
	}
}

func TestC39_CA_005_ConsensusAuditEnabledDefaultsTrue(t *testing.T) {
	dflt := policy.Policy{}.WorkflowConfig()
	if !dflt.ConsensusAuditEnabled {
		t.Errorf("RED: policy.Policy{}.WorkflowConfig().ConsensusAuditEnabled = false, want true.\n" +
			"Builder must add ConsensusAuditEnabled bool to WorkflowConfig with default=true\n" +
			"in WorkflowConfig() resolver (nil ConsensusAuditEnabled in WorkflowPolicy → on).")
	}

	f := false
	disabled := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			ConsensusAuditEnabled: &f,
		},
	}.WorkflowConfig()
	if disabled.ConsensusAuditEnabled {
		t.Errorf("RED: ConsensusAuditEnabled with *bool=false pointer = true, want false.\n" +
			"Builder must honor explicit ConsensusAuditEnabled=false in policy.json:\n" +
			"  if p.Workflow.ConsensusAuditEnabled != nil {\n" +
			"    c.ConsensusAuditEnabled = *p.Workflow.ConsensusAuditEnabled\n" +
			"  } else { c.ConsensusAuditEnabled = true }")
	}

	tr := true
	enabled := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			ConsensusAuditEnabled: &tr,
		},
	}.WorkflowConfig()
	if !enabled.ConsensusAuditEnabled {
		t.Errorf("RED: ConsensusAuditEnabled with *bool=true pointer = false, want true.\n" +
			"Builder must propagate explicit ConsensusAuditEnabled=true from WorkflowPolicy\n" +
			"through to WorkflowConfig via the pointer dereference in the resolver.")
	}
}

// acs-predicate: config-check
func TestC39_CA_NEG1_NoConsensusAuditIPCWrite(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	loopArgsFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_args.go")
	if !acsassert.FileNotContains(t, loopArgsFile, `"EVOLVE_CONSENSUS_AUDIT"`) {
		t.Errorf("RED: cmd_loop_args.go still contains the string literal \"EVOLVE_CONSENSUS_AUDIT\".\n"+
			"Builder must delete the line `out[\"EVOLVE_CONSENSUS_AUDIT\"] = \"1\"` at line ~270.\n"+
			"This IPC write was a no-op (default ConsensusEnvOff=false when env==\"\" or \"1\").\n"+
			"After migration, consensus audit is controlled via WorkflowPolicy.ConsensusAuditEnabled\n"+
			"in policy.json, not env injection.\n"+
			"File: %s", loopArgsFile)
	}
}
