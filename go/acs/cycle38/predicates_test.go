//go:build acs

package cycle38

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC38_001_RouterCLIAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_ROUTER_CLI"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove the EVOLVE_ROUTER_CLI row from registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_ROUTER_CLI", f.Status, f.Cluster)
	}
}

func TestC38_002_RouterModelAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_ROUTER_MODEL"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove the EVOLVE_ROUTER_MODEL row from registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_ROUTER_MODEL", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC38_005_NoProdRouterEnvReadsInCmdCycle(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")

	for _, name := range []string{"EVOLVE_ROUTER_CLI", "EVOLVE_ROUTER_MODEL"} {
		literal := `"` + name + `"`
		if !acsassert.FileNotContains(t, cmdFile, literal) {
			t.Errorf("RED: cmd_cycle.go still contains the string literal %s.\n"+
				"Builder must replace os.Getenv(%q) with rc.CLI or rc.Model from the\n"+
				"policy.RouterPolicy parameter (cycle-8 anti-gaming: removing the registry\n"+
				"row without deleting the os.Getenv call is the split-const hiding pattern).\n"+
				"File: %s", literal, name, cmdFile)
		}
	}
}

func TestC38_006_RouterPolicyHasCLIAndModelFields(t *testing.T) {
	cfg := policy.Policy{
		Router: &policy.RouterPolicy{
			CLI:   "codex-tmux",
			Model: "deep",
		},
	}.RouterConfig()

	if cfg.CLI != "codex-tmux" {
		t.Errorf("RED: RouterConfig().CLI = %q, want \"codex-tmux\".\n"+
			"Builder must add CLI string field to RouterPolicy and wire it in RouterConfig().\n"+
			"policy.RouterPolicy{CLI: \"codex-tmux\"} must flow through to the resolver.", cfg.CLI)
	}
	if cfg.Model != "deep" {
		t.Errorf("RED: RouterConfig().Model = %q, want \"deep\".\n"+
			"Builder must add Model string field to RouterPolicy and wire it in RouterConfig().\n"+
			"policy.RouterPolicy{Model: \"deep\"} must flow through to the resolver.", cfg.Model)
	}

	dflt := policy.Policy{}.RouterConfig()
	if dflt.CLI != "" {
		t.Errorf("RED: policy.Policy{}.RouterConfig().CLI = %q, want \"\" (no override default).\n"+
			"An absent/empty RouterPolicy.CLI must not override the profile default.", dflt.CLI)
	}
	if dflt.Model != "" {
		t.Errorf("RED: policy.Policy{}.RouterConfig().Model = %q, want \"\" (no override default).\n"+
			"An absent/empty RouterPolicy.Model must not override the profile default.", dflt.Model)
	}
}

func TestC38_008_ControlFlagsDocNoRouterCLIModelRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range []string{"EVOLVE_ROUTER_CLI", "EVOLVE_ROUTER_MODEL"} {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"the 2 router CLI/model rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

// acs-predicate: config-check
func TestC38_NEG1_NoStaleRouterEnvSetenvsInTests(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	testFiles := []string{
		filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle_test.go"),
		filepath.Join(root, "go", "cmd", "evolve", "cmd_router_dispatch_test.go"),
	}
	for _, path := range testFiles {
		for _, name := range []string{"EVOLVE_ROUTER_CLI", "EVOLVE_ROUTER_MODEL"} {
			setenvLiteral := `t.Setenv("` + name + `"`
			if !acsassert.FileNotContains(t, path, setenvLiteral) {
				t.Errorf("RED: %s still contains %q.\n"+
					"Builder must remove the t.Setenv call for %q and replace it with\n"+
					"policy.RouterPolicy{CLI: \"...\"} or policy.RouterPolicy{Model: \"...\"}\n"+
					"(tests for a deleted env override path must not remain).\n"+
					"File: %s", filepath.Base(path), setenvLiteral, name, path)
			}
		}
	}
}

func TestC38_GC_001_GCAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_GC"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove the EVOLVE_GC row from registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q Kind=%q",
			"EVOLVE_GC", f.Status, f.Cluster, f.Kind)
	}
}

// acs-predicate: config-check
func TestC38_GC_004_NoProdGCEnvReadInCmdLoopOutcome(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	outcomeFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_outcome.go")
	literal := `"EVOLVE_GC"`
	if !acsassert.FileNotContains(t, outcomeFile, literal) {
		t.Errorf("RED: cmd_loop_outcome.go still contains the string literal %s.\n"+
			"Builder must replace os.Getenv(\"EVOLVE_GC\") with gcPol.Mode after loading\n"+
			"the gc.Policy from policy.json (cycle-8 anti-gaming: removing the registry\n"+
			"row without deleting the os.Getenv call is the split-const hiding pattern).\n"+
			"File: %s", literal, outcomeFile)
	}
}

func TestC38_GC_005_GCPolicyHasModeField(t *testing.T) {
	pol := gc.Policy{Mode: "shadow"}
	if pol.Mode != "shadow" {
		t.Errorf("RED: gc.Policy{Mode: \"shadow\"}.Mode = %q, want \"shadow\".\n"+
			"Builder must add Mode string field to gc.Policy in go/internal/gc/gc.go.", pol.Mode)
	}

	zeroPol := gc.Policy{}
	if zeroPol.Mode != "" {
		t.Errorf("RED: gc.Policy{}.Mode = %q, want \"\" (zero value = off behavior).\n"+
			"The Mode field must be a plain string (zero value = \"\"); runGCHook converts\n"+
			"\"\" to \"off\" behavior so an absent gc block in policy.json behaves as disabled.", zeroPol.Mode)
	}
}

// acs-predicate: config-check
func TestC38_GC_008_ControlFlagsDocNoGCRow(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	if !acsassert.FileNotContains(t, controlFlagsDoc, "EVOLVE_GC") {
		t.Errorf("RED: control-flags.md still contains \"EVOLVE_GC\".\n"+
			"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
			"the EVOLVE_GC row (e.g. `evolve flags generate`) in the same diff.\n"+
			"File: %s", controlFlagsDoc)
	}
}
