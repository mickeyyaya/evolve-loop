//go:build acs

package cycle23

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var deadFlags = []string{
	"EVOLVE_CODEX_REQUIRE_FULL",
	"EVOLVE_QUOTA_DANGER_PCT",
	"EVOLVE_REQUIRE_TEAM_CONTEXT",
	"EVOLVE_RUN_TIMEOUT",
	"EVOLVE_TASK_MODE",
}

var deadFlagsWithAgentOrSkillRefs = []string{
	"EVOLVE_CODEX_REQUIRE_FULL",
	"EVOLVE_REQUIRE_TEAM_CONTEXT",
	"EVOLVE_RUN_TIMEOUT",
	"EVOLVE_TASK_MODE",
}

func TestC23_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-23 dead-flag-sweep).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC23_004_NoProductionReaderForDeadFlags(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	for _, flag := range deadFlags {
		envRead := `os.Getenv("` + flag + `")`
		_, _, code, _ := acsassert.SubprocessOutput(
			"grep", "-rn", envRead, goDir,
			"--include=*.go",
			"--exclude=*_test.go",
			"--exclude=registry_table.go",
		)
		if code == 0 {
			t.Errorf("production Go code reads %q via os.Getenv — must be absent.\n"+
				"These flags are dead (0 production readers per scout-report §Key Findings).\n"+
				"Do not add os.Getenv calls for removed flags.",
				flag)
		}
	}
}

func TestC23_005_ControlFlagsMdHasNoDeadRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, flag := range deadFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 5 dead flag rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}

func TestC23_006_WorktreePathStillInRegistry(t *testing.T) {
	const worktreePath = "EVOLVE_WORKTREE_PATH"
	if _, ok := flagregistry.Lookup(worktreePath); !ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned ok=false — WORKTREE_PATH removed.\n"+
			"Builder MUST NOT remove EVOLVE_WORKTREE_PATH from registry_table.go.\n"+
			"It is a live IPC handoff (agents/evolve-tester.md) pinned by C50_009.\n"+
			"Correct retirement is cluster-10 split-const (explicitly deferred).",
			worktreePath)
	}
}

func TestC23_007_NoRemovedFlagRefsInAgentsOrSkills(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	agentsDir := filepath.Join(root, "agents")
	skillsDir := filepath.Join(root, "skills")
	for _, flag := range deadFlagsWithAgentOrSkillRefs {
		for _, dir := range []struct {
			name string
			path string
		}{
			{"agents", agentsDir},
			{"skills", skillsDir},
		} {
			_, _, code, _ := acsassert.SubprocessOutput(
				"grep", "-rl", flag, dir.path,
			)
			if code == 0 {
				t.Errorf("RED: %s/ still contains a reference to %q.\n"+
					"Builder must remove all agent/skill references alongside the registry row removal.\n"+
					"Scout-report §Research→Implementation Map lists the exact files to clean.\n"+
					"Directory: %s", dir.name, flag, dir.path)
			}
		}
	}
}

func TestC23_008_ParseQuotaDangerPctRemovedFromHelpers(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	helpersFile := filepath.Join(root, "go", "internal", "subagent", "helpers.go")
	if !acsassert.FileNotContains(t, helpersFile, "ParseQuotaDangerPct") {
		t.Errorf("RED: go/internal/subagent/helpers.go still contains ParseQuotaDangerPct.\n"+
			"Builder must remove the dead function ParseQuotaDangerPct and its test\n"+
			"(TestParseQuotaDangerPct in helpers_test.go) in the same diff.\n"+
			"File: %s", helpersFile)
	}
}
