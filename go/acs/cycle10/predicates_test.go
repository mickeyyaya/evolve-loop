//go:build acs

package cycle10

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC10_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	deadFlags := []string{
		"EVOLVE_PROMPT_MAX_TOKENS",
		"EVOLVE_REAP_ORPHANS",
		"EVOLVE_TESTING",
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM",
		"EVOLVE_WORKTREE_PATH",
	}
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — dead flag still registered.\n"+
				"Builder must delete this row from registry_table.go (w1-dead-flags).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC10_004_TombstoneFlagsAbsentFromRegistry(t *testing.T) {
	tombstones := []string{
		"EVOLVE_ANTHROPIC_BASE_URL",
		"EVOLVE_HANG_CLASSIFIER",
		"EVOLVE_MODELCATALOG_AUTOREFRESH",
		"EVOLVE_MARKETPLACE_DIR",
		"EVOLVE_ADVISOR_DEPTH",
		"EVOLVE_DISABLE_WORKSPACE_GUARD",
		"EVOLVE_POLICY_BYPASS",
		"EVOLVE_PLATFORM",
		"EVOLVE_COMPOSE_PHASES",
	}
	for _, name := range tombstones {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — tombstone still registered.\n"+
				"Builder must delete this row from registry_table.go (w1-tombstones-and-compose).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

func TestC10_005_ComposePhasesEnvRefsAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	composeFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_compose.go")
	if !acsassert.FileNotContains(t, composeFile, "EVOLVE_COMPOSE_PHASES") {
		t.Errorf("RED: cmd_compose.go still references EVOLVE_COMPOSE_PHASES.\n"+
			"Builder must remove all os.Getenv/os.Setenv/os.Unsetenv calls for\n"+
			"EVOLVE_COMPOSE_PHASES and replace with ComposePhases bool in PhaseRequest.\n"+
			"File: %s", composeFile)
	}
}

func TestC10_006_EnvBridgesRemovedFromCmdFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file string
		flag string
	}{
		{filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go"), "EVOLVE_DISABLE_WORKSPACE_GUARD"},
		{filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go"), "EVOLVE_DISABLE_WORKSPACE_GUARD"},
		{filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go"), "EVOLVE_POLICY_BYPASS"},
		{filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go"), "EVOLVE_POLICY_BYPASS"},
	}
	for _, tc := range checks {
		if !acsassert.FileNotContains(t, tc.file, tc.flag) {
			t.Errorf("RED: %s still contains env bridge read for %s.\n"+
				"Builder must remove the cycleEnv[%q] bridge read.\n"+
				"File: %s",
				filepath.Base(tc.file), tc.flag, tc.flag, tc.file)
		}
	}
}
