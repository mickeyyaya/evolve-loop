//go:build acs

package cycle50

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC50A_001_CodexConfigPath_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CODEX_CONFIG_PATH"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (codex-config-path-di-50: bucket-3 DI migration).\n"+
			"The os.Getenv read must be removed from codex_pretrust.go:142; replace with cfg.codexConfigPath.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_CODEX_CONFIG_PATH", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC50A_002_CodexConfigPath_AbsentFromCodexPretrust(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "bridge", "codex_pretrust.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_CODEX_CONFIG_PATH"`) {
		t.Errorf("RED: codex_pretrust.go still contains the env read \"EVOLVE_CODEX_CONFIG_PATH\".\n"+
			"Builder must:\n"+
			"  1. Rename codexConfigPath() → defaultCodexConfigPath()\n"+
			"  2. Add resolveCodexConfigPath(cfg *Config) that returns cfg.codexConfigPath if non-empty,\n"+
			"     else falls back to defaultCodexConfigPath()\n"+
			"  3. Update pretrustCodexProjects to call resolveCodexConfigPath(cfg) instead of codexConfigPath()\n"+
			"  4. Remove the os.Getenv(\"EVOLVE_CODEX_CONFIG_PATH\") branch from the renamed function\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC50A_003_BridgeConfig_HasCodexConfigPathField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "bridge", "engine.go")
	if !acsassert.FileMatchesRegex(t, f, `codexConfigPath\s+string`) {
		t.Errorf("RED: engine.go does not contain 'codexConfigPath string' field on bridge.Config.\n"+
			"Builder must add an unexported string field to bridge.Config (engine.go):\n"+
			"  codexConfigPath string  // test seam: overrides resolved codex config path\n"+
			"Pattern: consistent with ArtifactTimeoutS int / BootOnly bool seam fields.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC50A_005_BridgeTests_NoSetenvCodexConfigPath(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	bridgeDir := filepath.Join(root, "go", "internal", "bridge")
	files := []string{
		"codex_pretrust_test.go",
		"codex_pretrust_amplify_test.go",
		"codex_pretrust_concurrent_test.go",
		"codex_pretrust_launch_test.go",
		"preflight_test.go",
	}
	for _, name := range files {
		p := filepath.Join(bridgeDir, name)
		if !acsassert.FileNotContains(t, p, `"EVOLVE_CODEX_CONFIG_PATH"`) {
			t.Errorf("RED: %s still contains t.Setenv(\"EVOLVE_CODEX_CONFIG_PATH\", ...).\n"+
				"Builder must replace t.Setenv(\"EVOLVE_CODEX_CONFIG_PATH\", path) with\n"+
				"setting cfg.codexConfigPath = path on the bridge.Config struct already\n"+
				"constructed in that test. Cycle-41 lesson: ALL 5 files must be updated atomically.\n"+
				"File: %s", name, p)
		}
	}
}

func TestC50A_NEG_RowCountAtMost51(t *testing.T) {
	got := len(flagregistry.All)
	if got > 51 {
		t.Errorf("RED: len(flagregistry.All) = %d, want ≤ 51 (52 − 1 Task A flag).\n"+
			"Builder must remove exactly this 1 row from registry_table.go:\n"+
			"  EVOLVE_CODEX_CONFIG_PATH\n"+
			"Current count %d exceeds 51 — Task A flag not yet removed.",
			got, got)
	}
}

func TestC50B_001_ReleaseStrictPass_AbsentFromRegistry(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_RELEASE_STRICT_PASS"); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
			"Builder must remove this row from registry_table.go (release-strict-pass-cli-50: bucket-4 CLI flag migration).\n"+
			"Both os.Getenv reads must be removed: cmd_release_preflight.go:55 and releasepipeline/bridges.go:26.\n"+
			"Replace with --strict-pass CLI flag and releasepipeline.Options.StrictPass bool.\n"+
			"Current entry: Status=%q Cluster=%q",
			"EVOLVE_RELEASE_STRICT_PASS", f.Status, f.Cluster)
	}
}

// acs-predicate: config-check
func TestC50B_002_ReleaseStrictPass_AbsentFromReleasePreflight(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "cli", "opscmd", "release_preflight.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RELEASE_STRICT_PASS"`) {
		t.Errorf("RED: cmd_release_preflight.go still contains os.Getenv(\"EVOLVE_RELEASE_STRICT_PASS\").\n"+
			"Builder must:\n"+
			"  1. Add --strict-pass to the arg-parsing loop\n"+
			"  2. Replace: strictPass := os.Getenv(\"EVOLVE_RELEASE_STRICT_PASS\") == \"1\"\n"+
			"     With:    strictPass parsed from --strict-pass flag (var strictPass bool)\n"+
			"Pattern: follows --force-fresh (cycle-49), --skip-tests, --dry-run precedents.\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC50B_003_ReleaseStrictPass_AbsentFromBridges(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "releasepipeline", "bridges.go")
	if !acsassert.FileNotContains(t, f, `"EVOLVE_RELEASE_STRICT_PASS"`) {
		t.Errorf("RED: releasepipeline/bridges.go still contains os.Getenv(\"EVOLVE_RELEASE_STRICT_PASS\").\n"+
			"Builder must:\n"+
			"  1. Add strictPass bool param to runPreflightLib (4-arg → 5-arg)\n"+
			"  2. Replace: StrictPass: os.Getenv(\"EVOLVE_RELEASE_STRICT_PASS\") == \"1\"\n"+
			"     With:    StrictPass: strictPass\n"+
			"  3. Update the call site in releasepipeline.go to pass opts.StrictPass\n"+
			"  4. Update bridges_test.go call site to pass false as the new 5th arg\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC50B_005_StrictPassFlag_RegisteredInPreflight(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "cli", "opscmd", "release_preflight.go")
	if !acsassert.FileContains(t, f, `"strict-pass"`) {
		t.Errorf("RED: cmd_release_preflight.go does not contain the --strict-pass flag registration.\n"+
			"Builder must add a --strict-pass flag to the arg-parsing loop in cmd_release_preflight.go.\n"+
			"Pattern: follows --force-fresh (cmd_loop_args.go, cycle-49).\n"+
			"File: %s", f)
	}
}

// acs-predicate: config-check
func TestC50B_006_ReleasePipelineOptions_HasStrictPassField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "releasepipeline", "releasepipeline.go")
	if !acsassert.FileContains(t, f, "StrictPass bool") {
		t.Errorf("RED: releasepipeline.go Options struct does not contain 'StrictPass bool'.\n"+
			"Builder must add StrictPass bool to releasepipeline.Options and wire it through:\n"+
			"  releasepipeline.Options.StrictPass → runPreflightLib 5th arg → releasepreflight.Options.StrictPass\n"+
			"Verify the call site at releasepipeline.go:547 passes opts.StrictPass.\n"+
			"File: %s", f)
	}
}
