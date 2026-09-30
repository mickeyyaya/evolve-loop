//go:build acs

package cycle37

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclehealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_ROUTER_REPLAN",
	"EVOLVE_ROUTING_JUDGE",
	"EVOLVE_ROUTER_RECON_DIGEST",
	"EVOLVE_ROUTER_REPLAN_DEPTH",
	"EVOLVE_ROUTER_PLAN_MODEL",
	"EVOLVE_ROUTER_PROPOSE_MODEL",
}

func TestC37_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-37 router-config-cluster-37).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC37_004_NoRouterEnvReadsInSourceFiles(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	configFile := filepath.Join(root, "go", "internal", "config", "config.go")
	cmdFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")

	configFlags := []string{
		"EVOLVE_ROUTER_REPLAN",
		"EVOLVE_ROUTING_JUDGE",
		"EVOLVE_ROUTER_RECON_DIGEST",
		"EVOLVE_ROUTER_REPLAN_DEPTH",
	}
	for _, name := range configFlags {
		if !acsassert.FileNotContains(t, configFile, name) {
			t.Errorf("RED: config.go still contains the string %q.\n"+
				"Builder must delete the entire if-block for env[%q] in applyEnv\n"+
				"(cycle-8 anti-gaming: removing the registry row without deleting the env read\n"+
				"is the split-const hiding pattern).\n"+
				"File: %s", name, name, configFile)
		}
	}

	cmdFlags := []string{
		"EVOLVE_ROUTER_PLAN_MODEL",
		"EVOLVE_ROUTER_PROPOSE_MODEL",
	}
	for _, name := range cmdFlags {
		if !acsassert.FileNotContains(t, cmdFile, name) {
			t.Errorf("RED: cmd_cycle.go still contains the string %q.\n"+
				"Builder must replace os.Getenv(%q) with rc.PlanModel/rc.ProposeModel\n"+
				"from the policy.RouterConfig parameter (cycle-37 fix for cmd_cycle.go).\n"+
				"File: %s", name, name, cmdFile)
		}
	}
}

func TestC37_005_RouterConfigDefaults(t *testing.T) {
	cfg := policy.Policy{}.RouterConfig()

	if cfg.RouterReplan != "shadow" {
		t.Errorf("RED: RouterConfig().RouterReplan = %q, want \"shadow\".\n"+
			"RouterPolicy.RouterReplan default must be \"shadow\", matching\n"+
			"config.defaults() RouterReplan: StageShadow (ADR-0052 WS0-S1).",
			cfg.RouterReplan)
	}

	if cfg.RoutingJudge != false {
		t.Errorf("RED: RouterConfig().RoutingJudge = %v, want false.\n"+
			"RouterPolicy.RoutingJudge default must be false (off by default).",
			cfg.RoutingJudge)
	}

	if cfg.ReconDigest != false {
		t.Errorf("RED: RouterConfig().ReconDigest = %v, want false.\n"+
			"RouterPolicy.ReconDigest default must be false (off by default).",
			cfg.ReconDigest)
	}

	if cfg.ReplanDepth != 1 {
		t.Errorf("RED: RouterConfig().ReplanDepth = %d, want 1.\n"+
			"RouterPolicy.ReplanDepth default must be 1, matching\n"+
			"config.defaults() RePlanMaxDepth (EVOLVE_ROUTER_REPLAN_DEPTH default).",
			cfg.ReplanDepth)
	}

	if cfg.PlanModel != "" {
		t.Errorf("RED: RouterConfig().PlanModel = %q, want \"\".\n"+
			"RouterPolicy.PlanModel default must be empty string (no override).",
			cfg.PlanModel)
	}

	if cfg.ProposeModel != "" {
		t.Errorf("RED: RouterConfig().ProposeModel = %q, want \"\".\n"+
			"RouterPolicy.ProposeModel default must be empty string (no override).",
			cfg.ProposeModel)
	}
}

func TestC37_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

func TestC37_008_ControlFlagsDocNoRemovedFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing the\n"+
				"6 router flag rows (e.g. `evolve flags generate`) in the same diff.\n"+
				"File: %s", name, controlFlagsDoc)
		}
	}
}

func TestC37_NEG1_StaleConfigTestFilesDeleted(t *testing.T) {
	root := acsassert.RepoRoot(t)
	staleFiles := []string{
		"go/internal/config/router_replan_test.go",
		"go/internal/config/routing_judge_test.go",
		"go/internal/config/router_recon_test.go",
	}
	for _, rel := range staleFiles {
		abs := filepath.Join(root, rel)
		if _, err := os.Stat(abs); err == nil {
			t.Errorf("RED: stale test file %q still exists on disk.\n"+
				"This file tests the now-deleted env override path for the migrated flag.\n"+
				"Builder must DELETE this file in the same diff as the registry row removals\n"+
				"(this was the cycle-35 root cause: stale test files caused full-suite red).\n"+
				"Path: %s", rel, abs)
		}
	}
}

type phaseTimingEntry struct {
	Phase       string `json:"phase"`
	Verdict     string `json:"verdict"`
	AbortReason string `json:"abort_reason,omitempty"`
}

func TestC37_009_EmptyWorkspaceClassifiesExplained(t *testing.T) {
	emptyWorkspace := t.TempDir()
	outcome, detail := cyclehealth.ClassifyOutcome(emptyWorkspace)
	if outcome == cyclehealth.OutcomeFailedUnexplained {
		t.Errorf("RED: ClassifyOutcome(empty workspace) = %s (detail: %q).\n"+
			"An empty workspace is produced when newCycleRun fails before any phase runs\n"+
			"(the cycle-0 FAILED_UNEXPLAINED incident). After the fix, this must return\n"+
			"FAILED_EXPLAINED (or another non-UNEXPLAINED outcome) — the C1 chokepoint\n"+
			"must cover the init-failure escaping path (ADR-0044 §C1).\n"+
			"Builder must route the escaping path through outcome recording.\n"+
			"See: unexplained-outcome-cycle-0 triage item.", outcome, detail)
	}
}

func TestC37_NEG2_ShipPassWorkspaceClassifiesShipped(t *testing.T) {
	ws := t.TempDir()
	timingEntries := []phaseTimingEntry{
		{Phase: "ship", Verdict: "PASS"},
	}
	raw, err := json.Marshal(timingEntries)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "phase-timing.json"), raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	outcome, _ := cyclehealth.ClassifyOutcome(ws)
	if outcome != cyclehealth.OutcomeShipped {
		t.Errorf("REGRESSION: ClassifyOutcome with ship PASS timing = %s, want SHIPPED.\n"+
			"The fix for unexplained-outcome-cycle-0 must not break the primary\n"+
			"SHIPPED classification path.", outcome)
	}
}
