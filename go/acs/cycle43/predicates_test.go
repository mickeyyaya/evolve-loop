//go:build acs

package cycle43

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_SCROLLBACK_LINES",
	"EVOLVE_BOOT_TIMEOUT_S",
	"EVOLVE_ARTIFACT_TIMEOUT_S",
	"EVOLVE_ARTIFACT_MAX_EXTENDS",
	"EVOLVE_PSMAS_SKIP",
}

func TestC43_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go\n"+
				"(bridge-timing-psmas-config-43: migrate to BridgePolicy/WorkflowPolicy).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC43_002_NoProdEnvReadsForRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	prodFiles := []struct {
		relPath string
		flags   []string
	}{
		{
			relPath: "go/internal/bridge/driver_tmux_repl.go",
			flags:   []string{"EVOLVE_SCROLLBACK_LINES", "EVOLVE_BOOT_TIMEOUT_S", "EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/recipe_adapter.go",
			flags:   []string{"EVOLVE_BOOT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/engine.go",
			flags:   []string{"EVOLVE_ARTIFACT_MAX_EXTENDS"},
		},
		{
			relPath: "go/cmd/evolve/cmd_phase_observer.go",
			flags:   []string{"EVOLVE_ARTIFACT_MAX_EXTENDS"},
		},
		{
			relPath: "go/internal/core/cyclerun.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
		{
			relPath: "go/internal/core/cyclerun_record.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
		{
			relPath: "go/internal/core/cyclerun_select.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
	}

	for _, pf := range prodFiles {
		fullPath := filepath.Join(root, pf.relPath)
		for _, flag := range pf.flags {
			if !acsassert.FileNotContains(t, fullPath, flag) {
				t.Errorf("RED: %s still contains the env flag string %q.\n"+
					"Builder must replace all envInt/envchain reads for %q with the typed\n"+
					"BridgePolicy/WorkflowPolicy field accessor (cycle-8 anti-gaming:\n"+
					"removing the registry row without deleting the env read is split-const hiding).\n"+
					"File: %s", pf.relPath, flag, flag, fullPath)
			}
		}
	}
}

func TestC43_003_BridgePolicyHas4IntTimingFields(t *testing.T) {
	cfg := policy.Policy{
		Bridge: &policy.BridgePolicy{
			BootTimeoutS:       90,
			ArtifactTimeoutS:   180,
			ArtifactMaxExtends: 5,
			ScrollbackLines:    3000,
		},
	}.BridgeConfig()

	if cfg.BootTimeoutS != 90 {
		t.Errorf("RED: BridgeConfig().BootTimeoutS = %d, want 90.\n"+
			"Builder must add BootTimeoutS int to BridgePolicy AND BridgeConfig,\n"+
			"then propagate in BridgeConfig() resolver.\n"+
			"(replaces EVOLVE_BOOT_TIMEOUT_S envInt reads in driver_tmux_repl.go and recipe_adapter.go)",
			cfg.BootTimeoutS)
	}
	if cfg.ArtifactTimeoutS != 180 {
		t.Errorf("RED: BridgeConfig().ArtifactTimeoutS = %d, want 180.\n"+
			"Builder must add ArtifactTimeoutS int to BridgePolicy AND BridgeConfig,\n"+
			"then propagate in BridgeConfig() resolver.\n"+
			"(replaces EVOLVE_ARTIFACT_TIMEOUT_S envInt reads in driver_tmux_repl.go)",
			cfg.ArtifactTimeoutS)
	}
	if cfg.ArtifactMaxExtends != 5 {
		t.Errorf("RED: BridgeConfig().ArtifactMaxExtends = %d, want 5.\n"+
			"Builder must add ArtifactMaxExtends int to BridgePolicy AND BridgeConfig,\n"+
			"then propagate in BridgeConfig() resolver.\n"+
			"(replaces EVOLVE_ARTIFACT_MAX_EXTENDS envInt reads in engine.go and cmd_phase_observer.go)",
			cfg.ArtifactMaxExtends)
	}
	if cfg.ScrollbackLines != 3000 {
		t.Errorf("RED: BridgeConfig().ScrollbackLines = %d, want 3000.\n"+
			"Builder must add ScrollbackLines int to BridgePolicy AND BridgeConfig,\n"+
			"then propagate in BridgeConfig() resolver.\n"+
			"(replaces EVOLVE_SCROLLBACK_LINES envInt reads in driver_tmux_repl.go)",
			cfg.ScrollbackLines)
	}

	dflt := policy.Policy{}.BridgeConfig()
	if dflt.BootTimeoutS != 0 {
		t.Errorf("RED: policy.Policy{}.BridgeConfig().BootTimeoutS = %d, want 0 (absent block → zero).\n"+
			"An absent/empty Bridge block must not override timing defaults; 0 tells the driver\n"+
			"to use its built-in constant (tmuxREPLBootTimeoutS=60 / tmuxArtifactScrollback / etc.).",
			dflt.BootTimeoutS)
	}
}

func TestC43_004_WorkflowPolicyHasPSMASEnabledBool(t *testing.T) {
	dflt := policy.Policy{}.WorkflowConfig()
	if dflt.PSMASEnabled {
		t.Errorf("RED: policy.Policy{}.WorkflowConfig().PSMASEnabled = true, want false.\n" +
			"Builder must add PSMASEnabled bool to WorkflowConfig with default=false in\n" +
			"WorkflowConfig() resolver (nil PSMASEnabled in WorkflowPolicy → off; PSMAS is opt-in).")
	}

	tr := true
	enabled := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			PSMASEnabled: &tr,
		},
	}.WorkflowConfig()
	if !enabled.PSMASEnabled {
		t.Errorf("RED: PSMASEnabled with *bool=true pointer = false, want true.\n" +
			"Builder must honor explicit PSMASEnabled=true in policy.json:\n" +
			"  if p.Workflow.PSMASEnabled != nil {\n" +
			"    c.PSMASEnabled = *p.Workflow.PSMASEnabled\n" +
			"  }")
	}

	f := false
	disabled := policy.Policy{
		Workflow: &policy.WorkflowPolicy{
			PSMASEnabled: &f,
		},
	}.WorkflowConfig()
	if disabled.PSMASEnabled {
		t.Errorf("RED: PSMASEnabled with *bool=false pointer = true, want false.\n" +
			"Builder must propagate explicit PSMASEnabled=false from WorkflowPolicy.")
	}
}

func TestC43_005_BridgeDepsHas4TypedIntFields(t *testing.T) {
	d := bridge.Deps{
		ScrollbackLines:    2000,
		BootTimeoutS:       45,
		ArtifactTimeoutS:   600,
		ArtifactMaxExtends: 3,
	}

	if d.ScrollbackLines != 2000 {
		t.Errorf("RED: bridge.Deps.ScrollbackLines = %d, want 2000.\n"+
			"Builder must add ScrollbackLines int to bridge.Deps in engine.go.",
			d.ScrollbackLines)
	}
	if d.BootTimeoutS != 45 {
		t.Errorf("RED: bridge.Deps.BootTimeoutS = %d, want 45.\n"+
			"Builder must add BootTimeoutS int to bridge.Deps in engine.go.",
			d.BootTimeoutS)
	}
	if d.ArtifactTimeoutS != 600 {
		t.Errorf("RED: bridge.Deps.ArtifactTimeoutS = %d, want 600.\n"+
			"Builder must add ArtifactTimeoutS int to bridge.Deps in engine.go\n"+
			"(currently only in Config, not Deps; test files set via Deps.Env map today).",
			d.ArtifactTimeoutS)
	}
	if d.ArtifactMaxExtends != 3 {
		t.Errorf("RED: bridge.Deps.ArtifactMaxExtends = %d, want 3.\n"+
			"Builder must add ArtifactMaxExtends int to bridge.Deps in engine.go.",
			d.ArtifactMaxExtends)
	}
}

func TestC43_010_ControlFlagsDocNoRemovedFlagRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"the 5 flag rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

// acs-predicate: config-check
func TestC43_011_CIWorkflowNoBootTimeoutOverride(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	goYML := filepath.Join(root, ".github", "workflows", "go.yml")
	if !acsassert.FileNotContains(t, goYML, "EVOLVE_BOOT_TIMEOUT_S") {
		t.Errorf("RED: .github/workflows/go.yml still contains EVOLVE_BOOT_TIMEOUT_S.\n"+
			"Builder must remove the `EVOLVE_BOOT_TIMEOUT_S: \"120\"` line from the CI workflow.\n"+
			"After migration, boot timeout is set via BridgePolicy.BootTimeoutS in policy.json;\n"+
			"the env CI override is dead code and the flagreaders guard will catch it.\n"+
			"File: %s", goYML)
	}
}

// acs-predicate: config-check
func TestC43_012_AllBridgeTestFilesMigrated(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	testFiles := []struct {
		relPath string
		flags   []string
	}{
		{
			relPath: "go/internal/bridge/scrollback_lines_test.go",
			flags:   []string{"EVOLVE_SCROLLBACK_LINES"},
		},
		{
			relPath: "go/internal/bridge/driver_tmux_repl_boottimeout_test.go",
			flags:   []string{"EVOLVE_BOOT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/envoverlay_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/render_wedge_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/driver_tmux_repl_escalation_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/tmux_repl_fixture_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_MAX_EXTENDS"},
		},
		{
			relPath: "go/internal/bridge/stop_review_ledger_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/bridge/stopreview_test.go",
			flags:   []string{"EVOLVE_ARTIFACT_TIMEOUT_S"},
		},
		{
			relPath: "go/internal/core/cyclerun.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
		{
			relPath: "go/internal/core/cyclerun_record.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
		{
			relPath: "go/internal/core/cyclerun_select.go",
			flags:   []string{"EVOLVE_PSMAS_SKIP"},
		},
	}

	for _, tf := range testFiles {
		fullPath := filepath.Join(root, tf.relPath)
		for _, flag := range tf.flags {
			if !acsassert.FileNotContains(t, fullPath, flag) {
				t.Errorf("RED: %s still contains the env flag string %q.\n"+
					"Builder must replace `Deps{Env: map[string]string{%q: \"N\"}}` (or LookupEnv map)\n"+
					"with the typed Deps field directly (e.g. `Deps{ArtifactTimeoutS: N}`).\n"+
					"This is the cycle-42 failure point: test migration was incomplete.\n"+
					"File: %s", tf.relPath, flag, flag, fullPath)
			}
		}
	}
}

// acs-predicate: config-check
func TestC43_NEG_PSMASAbsentFromCyclerunProdFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	psmasFlag := "EVOLVE_PSMAS_SKIP"

	cyclerunFiles := []string{
		"go/internal/core/cyclerun.go",
		"go/internal/core/cyclerun_record.go",
		"go/internal/core/cyclerun_select.go",
	}

	for _, rel := range cyclerunFiles {
		fullPath := filepath.Join(root, rel)
		if !acsassert.FileNotContains(t, fullPath, psmasFlag) {
			t.Errorf("RED: %s still contains the string literal %q.\n"+
				"Builder must replace envchain.BoolValue(cr.envSnap[\"EVOLVE_PSMAS_SKIP\"], false)\n"+
				"with pol.WorkflowConfig().PSMASEnabled (or equivalent policy read).\n"+
				"(cycle-8 anti-gaming: removing the registry row without deleting the env read\n"+
				"is the split-const hiding pattern).\n"+
				"File: %s", rel, psmasFlag, fullPath)
		}
	}
}
