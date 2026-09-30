//go:build acs

package cycle34

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_CONTRACT_GATE",
	"EVOLVE_EVAL_GATE",
	"EVOLVE_REVIEW_GATE",
	"EVOLVE_TRIAGE_CAP_GATE",
}

func TestC34_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-34 gates-config-cluster-34).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC34_004_NoGateEnvReadsInConfigGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	configFile := filepath.Join(root, "go", "internal", "config", "config.go")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, configFile, name) {
			t.Errorf("RED: config.go still contains the string %q.\n"+
				"Builder must delete the entire if-block for env[%q] in applyEnv\n"+
				"(cycle-8 anti-gaming: removing the registry row without deleting the env read\n"+
				"is the split-const hiding pattern).\n"+
				"File: %s", name, name, configFile)
		}
	}
}

func TestC34_005_GatesConfigDefaults(t *testing.T) {
	cfg := policy.Policy{}.GatesConfig()

	if cfg.ContractGate != "enforce" {
		t.Errorf("RED: GatesConfig().ContractGate = %q, want \"enforce\".\n"+
			"GatesPolicy.ContractGate default must be \"enforce\", matching\n"+
			"config.defaults() ContractGate: StageEnforce behavior.",
			cfg.ContractGate)
	}

	if cfg.EvalGate != "enforce" {
		t.Errorf("RED: GatesConfig().EvalGate = %q, want \"enforce\".\n"+
			"GatesPolicy.EvalGate default must be \"enforce\", matching\n"+
			"config.defaults() EvalGate: StageEnforce behavior.",
			cfg.EvalGate)
	}

	if cfg.TriageCapGate != "enforce" {
		t.Errorf("RED: GatesConfig().TriageCapGate = %q, want \"enforce\".\n"+
			"GatesPolicy.TriageCapGate default must be \"enforce\", matching\n"+
			"config.defaults() TriageCapGate: StageEnforce behavior.",
			cfg.TriageCapGate)
	}

	if cfg.ReviewGate != "off" {
		t.Errorf("RED: GatesConfig().ReviewGate = %q, want \"off\".\n"+
			"GatesPolicy.ReviewGate default must be \"off\", matching\n"+
			"config.defaults() ReviewGate: StageOff (zero value) behavior.",
			cfg.ReviewGate)
	}
}

func TestC34_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

func TestC34_NEG1_DefaultContractGateIsEnforce(t *testing.T) {
	cfg, _ := config.Load("", map[string]string{})

	if cfg.ContractGate != config.StageEnforce {
		t.Errorf("RED: config.Load(emptyEnv).ContractGate = %v, want StageEnforce (%v).\n"+
			"Removing env reads from applyEnv must not change the default.\n"+
			"config.defaults() must still set ContractGate: StageEnforce.",
			cfg.ContractGate, config.StageEnforce)
	}

	if cfg.EvalGate != config.StageEnforce {
		t.Errorf("RED: config.Load(emptyEnv).EvalGate = %v, want StageEnforce (%v).\n"+
			"Removing env reads from applyEnv must not change the default.",
			cfg.EvalGate, config.StageEnforce)
	}

	if cfg.TriageCapGate != config.StageEnforce {
		t.Errorf("RED: config.Load(emptyEnv).TriageCapGate = %v, want StageEnforce (%v).\n"+
			"Removing env reads from applyEnv must not change the default.",
			cfg.TriageCapGate, config.StageEnforce)
	}

	if cfg.ReviewGate != config.StageOff {
		t.Errorf("RED: config.Load(emptyEnv).ReviewGate = %v, want StageOff (%v).\n"+
			"ReviewGate is not set in defaults() so must be zero (StageOff) with empty env.",
			cfg.ReviewGate, config.StageOff)
	}
}
