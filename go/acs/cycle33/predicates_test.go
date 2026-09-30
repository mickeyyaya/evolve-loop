//go:build acs

package cycle33

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_PHASE_BUILD_BIN",
	"EVOLVE_SHIP_SCRIPT",
	"EVOLVE_SWARM_PORT_BASE",
	"EVOLVE_SWARM_STAGE",
}

func TestC33_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-33 swarm-config-cluster-33).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC33_004_SwarmStageNotInSwarmrunner(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	swarmrunnerFile := filepath.Join(root, "go", "internal", "phases", "swarmrunner", "swarmrunner.go")
	if !acsassert.FileNotContains(t, swarmrunnerFile, "EVOLVE_SWARM_STAGE") {
		t.Errorf("RED: swarmrunner.go still contains the string 'EVOLVE_SWARM_STAGE'.\n"+
			"Builder must delete the env map read (swarmStage(req.Env)) and update\n"+
			"the package doc comment — replacing with parseSwarmStage(d.cfg.Stage).\n"+
			"File: %s", swarmrunnerFile)
	}
}

// acs-predicate: config-check
func TestC33_005_SwarmPortBaseNotInSwarmrunner(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	swarmrunnerFile := filepath.Join(root, "go", "internal", "phases", "swarmrunner", "swarmrunner.go")
	if !acsassert.FileNotContains(t, swarmrunnerFile, "EVOLVE_SWARM_PORT_BASE") {
		t.Errorf("RED: swarmrunner.go still contains the string 'EVOLVE_SWARM_PORT_BASE'.\n"+
			"Builder must delete portBaseFromEnv(req.Env) from dispatchDeps() and\n"+
			"replace it with d.cfg.PortBase (DI from swarmrunner.Config).\n"+
			"File: %s", swarmrunnerFile)
	}
}

func TestC33_006_SwarmConfigDefaults(t *testing.T) {
	cfg := policy.Policy{}.SwarmConfig()

	if cfg.Stage != "shadow" {
		t.Errorf("RED: SwarmConfig().Stage = %q, want %q.\n"+
			"SwarmPolicy.Stage is a string; empty/nil must resolve to 'shadow',\n"+
			"matching the existing swarmStage() default → stageOff (shadow/delegate) behavior.\n"+
			"Builder must set: c.Stage = 'shadow' as the default in SwarmConfig().",
			cfg.Stage, "shadow")
	}

	if cfg.PortBase != 0 {
		t.Errorf("RED: SwarmConfig().PortBase = %d, want 0.\n"+
			"PortBase is an int; the zero value is the correct default, matching\n"+
			"portBaseFromEnv's 'Unset/invalid → 0' behavior.\n"+
			"Builder must NOT set a non-zero PortBase default in SwarmConfig().",
			cfg.PortBase)
	}
}

func TestC33_008_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// acs-predicate: config-check — doc regeneration is a required build step.
func TestC33_009_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 4 rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

// acs-predicate: config-check
func TestC33_NEG1_PortBaseFromEnvDeleted(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	swarmrunnerFile := filepath.Join(root, "go", "internal", "phases", "swarmrunner", "swarmrunner.go")
	if !acsassert.FileNotContains(t, swarmrunnerFile, "portBaseFromEnv") {
		t.Errorf("RED: swarmrunner.go still contains the 'portBaseFromEnv' function.\n"+
			"Builder must DELETE portBaseFromEnv() (not just stop calling it)\n"+
			"after replacing the call in dispatchDeps() with d.cfg.PortBase.\n"+
			"The function is the sole env-read mechanism for port base; its presence means\n"+
			"the migration is incomplete even if the direct caller is updated.\n"+
			"File: %s", swarmrunnerFile)
	}
}

// acs-predicate: config-check
func TestC33_NEG2_SwarmConfigUsedAtCompositionRoot(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdCycleFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")
	if !acsassert.FileContains(t, cmdCycleFile, "swarmrunner.Config") {
		t.Errorf("RED: cmd_cycle.go does not contain 'swarmrunner.Config'.\n"+
			"Builder must define swCfg := swarmrunner.Config{Stage: pol.SwarmConfig().Stage,\n"+
			"PortBase: pol.SwarmConfig().PortBase} and pass swCfg as the 4th arg to\n"+
			"both swarmrunner.New(...) calls (~lines 289 and 293).\n"+
			"File: %s", cmdCycleFile)
	}
}
