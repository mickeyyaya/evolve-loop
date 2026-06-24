//go:build acs

// Package cycle356 materializes the cycle-356 acceptance criteria for the
// committed top_n task:
//
//   - budget-cluster-dead-flag-removal — remove 12 dead Budget Cluster flags
//     from registry_table.go, clean production Go references and help text,
//     remove ErrBudgetExceeded dead code, clean skills/docs, regenerate
//     control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	budget-cluster-dead-flag-removal:
//	  AC-1 (neg)  12 Budget Cluster flags absent from flagregistry.Lookup → C356_001
//	  AC-2        TestFlagRegistry_NoBudgetClusterDeadFlags passes          → C356_002
//	  AC-3        EVOLVE_BUILD_PLANNER preserved (not removed)              → C356_003
//	  AC-4        evolve flags check exits 0 (no drift)                     → C356_004
//	  AC-5 (neg)  ErrBudgetExceeded absent from go/internal/core/errors.go  → C356_005
//	  AC-5 (neg)  No EVOLVE_FANOUT_PER_WORKER_BUDGET_USD in cmd help text   → C356_006
//	  AC-6 (neg)  Skills/docs cleaned of budget flag references             → C356_007
//
// Floor binding (R9.3): only committed top_n items get predicates.
// All deferred tasks (deprecated-bridge-retirement, etc.) get zero predicates.
package cycle356

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the go module directory for subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// budgetClusterFlags is the complete list of the 12 StatusDead Budget Cluster
// flags that cycle-356 removes. Keep in sync with scout-report.md.
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

// TestC356_001_BudgetClusterFlagsAbsentFromLookup verifies that all 12 dead
// Budget Cluster flags are no longer present in the flagregistry after Builder
// removes them from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly (the production SSOT
// binary-search function). Source edits alone cannot satisfy this — the flag
// row must be physically absent from registry_table.go for Lookup to return
// ok=false.
//
// NEGATIVE: each assertion requires ok==false. Before Builder's change, all
// 12 flags ARE in the registry (ok==true), so the test fails for all 12.
//
// RED: all 12 flags are currently StatusDead in registry_table.go.
// Lookup("EVOLVE_BATCH_BUDGET_CAP") returns ok=true → assert !ok fails.
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

// TestC356_002_RegressionGuardTestPassesInFlagRegistry verifies that the new
// TestFlagRegistry_NoBudgetClusterDeadFlags regression guard test exists and
// passes in the flagregistry package.
//
// BEHAVIORAL: runs the actual go test binary against the flagregistry package.
// Source edits alone cannot satisfy this — the test must be authored AND the
// 12 flags must be removed for it to pass.
//
// RED: before Builder's work, TestFlagRegistry_NoBudgetClusterDeadFlags
// either does not exist (compile error / test not found) or fails because
// the 12 flags are still present.
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

// TestC356_004_FlagsCheckExitsZero verifies that `evolve flags check` exits 0,
// confirming that the Generated Flag Index in docs/architecture/control-flags.md
// is in sync with the flagregistry after Builder removes the 12 dead flags and
// runs `evolve flags generate`.
//
// BEHAVIORAL: runs the real evolve binary; registry edits alone cannot satisfy it.
//
// NOTE: this predicate is pre-existing GREEN in the current (pre-Builder) state
// because the flags are still present in both the registry and control-flags.md.
// It becomes RED mid-Builder-work (after registry row removal, before
// regeneration) and GREEN again after `evolve flags generate` is re-run.
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

// TestC356_005_ErrBudgetExceededAbsentFromCoreErrors verifies that the dead
// ErrBudgetExceeded sentinel (go/internal/core/errors.go) was removed.
// ErrBudgetExceeded has no production caller — only errors_test.go referenced
// it as a sentinel, and that test entry must also be removed.
//
// NEGATIVE: the file must NOT contain "ErrBudgetExceeded" after Builder's change.
//
// RED: go/internal/core/errors.go currently defines ErrBudgetExceeded at line 20.
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

// TestC356_006_FanoutHelpTextNoBudgetFlag verifies that the cmd help text in
// cmd_subagent.go and cmd_fanout_dispatch.go no longer references the removed
// EVOLVE_FANOUT_PER_WORKER_BUDGET_USD flag string.
//
// NEGATIVE: both files must NOT contain "EVOLVE_FANOUT_PER_WORKER_BUDGET_USD".
//
// RED: currently cmd_subagent.go:50,376 and cmd_fanout_dispatch.go:25 contain
// the flag name in help-text Fprintln calls.
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

// TestC356_007_SkillsDocsNoBudgetClusterReferences verifies that the three skills
// documentation files no longer reference the removed Budget Cluster flags.
//
// NEGATIVE: all three files must not contain the budget flag references that
// Scout identified at specific lines.
//
// RED:
//   - skills/loop/SKILL.md:176,206 contain EVOLVE_BATCH_BUDGET_CAP references.
//   - skills/loop/phases.md:227 contains EVOLVE_BUILDER_COST_THRESHOLD reference.
//   - skills/loop/reference/claude-runtime.md:57 contains EVOLVE_MAX_BUDGET_USD row.
//
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
