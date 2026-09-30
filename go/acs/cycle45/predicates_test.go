//go:build acs

package cycle45

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var allRemovedFlags = []string{
	"EVOLVE_GO_BIN_TEST",
	"EVOLVE_PLAN_WORKSPACE",
	"EVOLVE_CODEX_VERSION_PATH",
	"EVOLVE_COMPACT_PROMPTS",
}

func TestC45_001_GoBinTestAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_GO_BIN_TEST"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (gobin-planworkspace-di-45: DI inject).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_GO_BIN_TEST", f.Status, f.Cluster)
	}
}

func TestC45_002_PlanWorkspaceAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_PLAN_WORKSPACE"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (gobin-planworkspace-di-45: CLI flag).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_PLAN_WORKSPACE", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC45_003_GoBinTestAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "releasepreflight", "releasepreflight.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_GO_BIN_TEST"`) {
		t.Errorf("RED: releasepreflight.go still contains the env read \"EVOLVE_GO_BIN_TEST\".\n"+
			"Builder must delete: goBin := os.Getenv(\"EVOLVE_GO_BIN_TEST\") (line 215)\n"+
			"and replace defaultSimulationRunner's env read with a goBinFn func() string param.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC45_004_PlanWorkspaceAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "cmd_plan_and_execute.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_PLAN_WORKSPACE"`) {
		t.Errorf("RED: cmd_plan_and_execute.go still contains the env read \"EVOLVE_PLAN_WORKSPACE\".\n"+
			"Builder must delete the os.Getenv(\"EVOLVE_PLAN_WORKSPACE\") call (line 70)\n"+
			"and add a --workspace flag to the plan-and-execute flag set.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC45_005_PlanWorkspaceRemovedFromDocsContract(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "cmd", "evolve", "docs_contract_test.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_PLAN_WORKSPACE"`) {
		t.Errorf("RED: docs_contract_test.go still has \"EVOLVE_PLAN_WORKSPACE\" in allowedUndocumented.\n"+
			"Builder must remove the entry \"EVOLVE_PLAN_WORKSPACE\": true (line 58).\n"+
			"After removal from the registry the allowedUndocumented entry is stale.\n"+
			"File: %s", f)
	}
}

func TestC45B_001_CodexVersionPathAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CODEX_VERSION_PATH"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (codex-version-compact-di-45: DI var).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_CODEX_VERSION_PATH", f.Status, f.Cluster)
	}
}

func TestC45B_002_CompactPromptsAbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_COMPACT_PROMPTS"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (codex-version-compact-di-45: Config Object).\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_COMPACT_PROMPTS", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC45B_003_CodexVersionPathAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "bridge", "codex_pretrust.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_CODEX_VERSION_PATH"`) {
		t.Errorf("RED: codex_pretrust.go still contains the env read \"EVOLVE_CODEX_VERSION_PATH\".\n"+
			"Builder must delete the os.Getenv(\"EVOLVE_CODEX_VERSION_PATH\") call (line 153)\n"+
			"and add: var codexVersionPathFn func() (string, error) = defaultCodexVersionPath.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC45B_004_CompactPromptsAbsentFromProdSource(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_COMPACT_PROMPTS"`) {
		t.Errorf("RED: runner.go still contains the envchain.Bool(\"EVOLVE_COMPACT_PROMPTS\") call (line 277).\n"+
			"Builder must replace it with b.compactPrompts (sourced from opts.CompactPrompts in New()).\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC45_007_ControlFlagsDocClean(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	for _, name := range allRemovedFlags {
		if !acsassert.FileNotContains(t, controlFlagsDoc, name) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must regenerate docs/architecture/control-flags.md after removing\n"+
				"all 4 flag rows (run `evolve flags generate` in the same diff).\n"+
				"File path: %s", name, controlFlagsDoc)
		}
	}
}
