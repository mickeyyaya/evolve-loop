//go:build acs

package cycle32

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var removedFlags = []string{
	"EVOLVE_ALLOW_DEEP_RESEARCH",
	"EVOLVE_ALLOW_DOC_DELETE",
	"EVOLVE_BACKFILL_ENABLED",
	"EVOLVE_BUILD_PLANNER_LATENCY_CEILING_S",
	"EVOLVE_CYCLE_BUDGET",
}

func TestC32_001_RemovedFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range removedFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-32 workflow-internal-cluster-32).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// acs-predicate: config-check
func TestC32_004_NoEnvReadsInProductionGo(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		absents []string
	}{
		{
			filepath.Join(root, "go", "internal", "core", "cyclerun_dispatch.go"),
			[]string{"EVOLVE_BACKFILL_ENABLED"},
		},
		{
			filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go"),
			[]string{"EVOLVE_CYCLE_BUDGET"},
		},
		{
			filepath.Join(root, "go", "internal", "guards", "docdelete.go"),
			[]string{"EVOLVE_ALLOW_DOC_DELETE"},
		},
	}
	for _, c := range checks {
		for _, pattern := range c.absents {
			if !acsassert.FileNotContains(t, c.file, pattern) {
				t.Errorf("RED: %s still contains env read for %q.\n"+
					"Builder must remove the os.Getenv / envchain / envEnabled() call for this flag\n"+
					"and replace it with the WorkflowConfig field or DI bool parameter.\n"+
					"File: %s",
					filepath.Base(c.file), pattern, c.file)
			}
		}
	}
}

func TestC32_005_WorkflowConfigDefaults(t *testing.T) {
	cfg := policy.Policy{}.WorkflowConfig()

	if !cfg.BackfillEnabled {
		t.Errorf("RED: WorkflowConfig().BackfillEnabled = false, want true.\n" +
			"WorkflowPolicy.BackfillEnabled is *bool; nil must resolve to the default-on\n" +
			"value (true), matching envchain.BoolValue(cr.envSnap[\"EVOLVE_BACKFILL_ENABLED\"], true).\n" +
			"Builder must initialize the default: c.BackfillEnabled = true in WorkflowConfig().")
	}
}

func TestC32_006_WorktreePathStillRegistered(t *testing.T) {
	if _, ok := flagregistry.Lookup("EVOLVE_WORKTREE_PATH"); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH was removed.\n"+
			"This flag is on the FORBIDDEN-REPEAT list (cycles 17/18/19 fail history).\n"+
			"Builder must NOT touch EVOLVE_WORKTREE_PATH in registry_table.go.",
			"EVOLVE_WORKTREE_PATH")
	}
}

// acs-predicate: config-check — doc regeneration is a required build step.
func TestC32_008_ControlFlagsMdHasNoRemovedRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range removedFlags {
		if !acsassert.FileNotContains(t, controlFlags, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 5 rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", name, controlFlags)
		}
	}
}

// acs-predicate: config-check
func TestC32_NEG1_EnvEnabledHelperDeleted(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	helpersFile := filepath.Join(root, "go", "internal", "guards", "helpers.go")
	if !acsassert.FileNotContains(t, helpersFile, "envEnabled") {
		t.Errorf("RED: guards/helpers.go still contains the 'envEnabled' function.\n"+
			"Builder must DELETE the envEnabled() helper (not just remove callers)\n"+
			"after migrating both quota.go and docdelete.go callers to the DI bool params.\n"+
			"The helper is the sole env-enable mechanism for guards; its presence means\n"+
			"the migration is incomplete even if individual callers are updated.\n"+
			"File: %s", helpersFile)
	}
}
