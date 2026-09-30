//go:build acs

package cycle356

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

var budgetClusterFlags = []string{
	"EVOLVE_BATCH_BUDGET_CAP",
	"EVOLVE_BATCH_BUDGET_DISABLE",
	"EVOLVE_BUDGET_CAP",
	"EVOLVE_BUDGET_ENFORCE",
	"EVOLVE_BUDGET_MAX_CYCLES",
	"EVOLVE_BUILDER_COST_GUARD_STRICT",
	"EVOLVE_BUILDER_COST_THRESHOLD",
	"EVOLVE_CHECKPOINT_AT_PCT",
	"EVOLVE_CHECKPOINT_WARN_AT_PCT",
	"EVOLVE_FANOUT_PER_WORKER_BUDGET_USD",
	"EVOLVE_MAX_BUDGET_USD",
	"EVOLVE_PHASE_COST_CEILING",
}

func TestC356_001_BudgetClusterFlagsAbsentFromLookup(t *testing.T) {
	for _, name := range budgetClusterFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag is still registered.\n"+
				"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
				"Current entry: Status=%q Cluster=%q Doc=%q",
				name, f.Status, f.Cluster, f.Doc)
		}
	}
}

func TestC356_002_RegressionGuardTestPassesInFlagRegistry(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"./internal/flagregistry/...",
		"-run", "TestFlagRegistry_NoBudgetClusterDeadFlags",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/flagregistry/... -run TestFlagRegistry_NoBudgetClusterDeadFlags failed (exit=%d).\n"+
			"Builder must (1) write TestFlagRegistry_NoBudgetClusterDeadFlags in the flagregistry package\n"+
			"AND (2) remove all 12 dead Budget Cluster flags from registry_table.go.\n\nOutput:\n%s",
			code, combined)
	}
}

func TestC356_004_FlagsCheckExitsZero(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binPath := filepath.Join(root, "go", "bin", "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"bash", "-c", "cd "+root+" && "+binPath+" flags check",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("evolve flags check exited %d: %v\nOutput:\n%s\n"+
			"Builder must run `evolve flags generate` after removing the 12 registry rows "+
			"to regenerate docs/architecture/control-flags.md.",
			code, err, combined)
	}
}

// acs-predicate: config-check
func TestC356_005_ErrBudgetExceededAbsentFromCoreErrors(t *testing.T) {
	root := acsassert.RepoRoot(t)
	errorsGo := filepath.Join(root, "go", "internal", "core", "errors.go")
	if !acsassert.FileNotContains(t, errorsGo, "ErrBudgetExceeded") {
		t.Errorf("RED: go/internal/core/errors.go still defines ErrBudgetExceeded.\n"+
			"Builder must remove the ErrBudgetExceeded var declaration (lines 18-20) and its\n"+
			"corresponding entry from go/internal/core/errors_test.go.\n"+
			"ErrBudgetExceeded has no production caller (grep: zero hits excluding _test.go and errors.go).\n"+
			"File: %s", errorsGo)
	}
}

// acs-predicate: config-check
func TestC356_006_FanoutHelpTextNoBudgetFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmdSubagent := filepath.Join(root, "go", "cmd", "evolve", "cmd_subagent.go")
	cmdFanout := filepath.Join(root, "go", "cmd", "evolve", "cmd_fanout_dispatch.go")

	if !acsassert.FileNotContains(t, cmdSubagent, "EVOLVE_FANOUT_PER_WORKER_BUDGET_USD") {
		t.Errorf("RED: go/cmd/evolve/cmd_subagent.go still references EVOLVE_FANOUT_PER_WORKER_BUDGET_USD "+
			"in help text.\nBuilder must remove the flag name from the dispatch-parallel help Fprintln "+
			"at lines 50 and 376.\nFile: %s", cmdSubagent)
	}
	if !acsassert.FileNotContains(t, cmdFanout, "EVOLVE_FANOUT_PER_WORKER_BUDGET_USD") {
		t.Errorf("RED: go/cmd/evolve/cmd_fanout_dispatch.go still references EVOLVE_FANOUT_PER_WORKER_BUDGET_USD "+
			"in help text.\nBuilder must remove the flag name from the Fprintln at line 25.\nFile: %s", cmdFanout)
	}
}

// acs-predicate: config-check
func TestC356_007_SkillsDocsNoBudgetClusterReferences(t *testing.T) {
	root := acsassert.RepoRoot(t)

	skillMD := filepath.Join(root, "skills", "loop", "SKILL.md")
	if !acsassert.FileNotContains(t, skillMD, "EVOLVE_BATCH_BUDGET_CAP") {
		t.Errorf("RED: skills/loop/SKILL.md still references EVOLVE_BATCH_BUDGET_CAP.\n"+
			"Builder must remove the two stale BATCH_BUDGET_CAP paragraphs at lines 176 and 206.\n"+
			"File: %s", skillMD)
	}

	phasesMD := filepath.Join(root, "skills", "loop", "phases.md")
	if !acsassert.FileNotContains(t, phasesMD, "EVOLVE_BUILDER_COST_THRESHOLD") {
		t.Errorf("RED: skills/loop/phases.md still references EVOLVE_BUILDER_COST_THRESHOLD.\n"+
			"Builder must remove the CostGuardDecorator budget-flag mention at line 227.\n"+
			"File: %s", phasesMD)
	}

	claudeRuntime := filepath.Join(root, "skills", "loop", "reference", "claude-runtime.md")
	if !acsassert.FileNotContains(t, claudeRuntime, "EVOLVE_MAX_BUDGET_USD") {
		t.Errorf("RED: skills/loop/reference/claude-runtime.md still contains the EVOLVE_MAX_BUDGET_USD row.\n"+
			"Builder must remove the table row at line 57.\n"+
			"File: %s", claudeRuntime)
	}
}
