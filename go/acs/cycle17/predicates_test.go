//go:build acs

package cycle17

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/research"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestLookup_CliMaxConcurrentCodexAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_CLI_MAX_CONCURRENT_CODEX")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_CLI_MAX_CONCURRENT_CODEX\") returned found=true.\n" +
			"Builder must delete the EVOLVE_CLI_MAX_CONCURRENT_CODEX row from\n" +
			"go/internal/flagregistry/registry_table.go (0 literal readers).\n" +
			"Do NOT remove the EVOLVE_CLI_MAX_CONCURRENT_ prefix in driver_tmux_repl.go\n" +
			"(that is a runtime-constructed name, not a literal reader).")
	}
}

func TestC17_101_CliMaxConcurrentCodexNoLiteralInRegistry(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	registryFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileNotContains(t, registryFile, "EVOLVE_CLI_MAX_CONCURRENT_CODEX") {
		t.Errorf("RED: registry_table.go still contains 'EVOLVE_CLI_MAX_CONCURRENT_CODEX'.\n" +
			"Builder must delete the row (currently around line 15 of registry_table.go).")
	}
}

func TestC17_110_CatalogDirNoOsGetenv(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	overlay := filepath.Join(root, "go", "internal", "bridge", "catalog_overlay.go")
	if !acsassert.FileNotContains(t, overlay, `os.Getenv("EVOLVE_MODEL_CATALOG_DIR")`) {
		t.Errorf("RED: catalog_overlay.go still calls os.Getenv(\"EVOLVE_MODEL_CATALOG_DIR\").\n" +
			"Builder must replace modelCatalogDir() func with:\n" +
			"  var modelCatalogDirFn = func() string { return policy.Load(...).BridgeConfig().CatalogDir }\n" +
			"matching capabilities.go's catalogDirFn pattern.\n" +
			"File: go/internal/bridge/catalog_overlay.go")
	}
}

func TestC17_111_CatalogDirNoOsSetenv(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	cmdCycle := filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go")
	if !acsassert.FileNotContains(t, cmdCycle, `os.Setenv("EVOLVE_MODEL_CATALOG_DIR"`) {
		t.Errorf("RED: cmd_cycle.go still calls os.Setenv(\"EVOLVE_MODEL_CATALOG_DIR\", ...).\n" +
			"Builder must replace this os.Setenv with a call to an exported setter\n" +
			"(e.g., bridge.SetModelCatalogDirFn(evolveDir)) that wires the fn-var\n" +
			"without touching the process environment.\n" +
			"File: go/cmd/evolve/cmd_cycle.go (currently line 245).")
	}
}

func TestLookup_ModelCatalogDirAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_MODEL_CATALOG_DIR")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_MODEL_CATALOG_DIR\") returned found=true.\n" +
			"Builder must delete the EVOLVE_MODEL_CATALOG_DIR row from registry_table.go\n" +
			"after replacing the os.Getenv/os.Setenv pair with fn-var DI.")
	}
}

func TestC17_113neg_CatalogDirFnVarInPlace(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	overlay := filepath.Join(root, "go", "internal", "bridge", "catalog_overlay.go")
	if !acsassert.FileContains(t, overlay, "modelCatalogDirFn") {
		t.Errorf("RED: catalog_overlay.go does not contain 'modelCatalogDirFn'.\n" +
			"Builder must replace the plain func modelCatalogDir() with a fn-var:\n" +
			"  var modelCatalogDirFn = func() string { return policy.Load(...).BridgeConfig().CatalogDir }\n" +
			"The existing capabilities.go file shows the same pattern (catalogDirFn).\n" +
			"File: go/internal/bridge/catalog_overlay.go")
	}
}

func TestC17_120_AcsSuiteNoEnvGetenv(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	suite := filepath.Join(root, "go", "internal", "acssuite", "acssuite.go")
	if !acsassert.FileNotContains(t, suite, `envGet("EVOLVE_ACS_GO_TIMEOUT_S")`) {
		t.Errorf("RED: acssuite.go still calls envGet(\"EVOLVE_ACS_GO_TIMEOUT_S\").\n" +
			"Builder must add ACSConfig{GoTimeoutS int} to policy.go and wire\n" +
			"goLaneTimeout to use ACSConfig.GoTimeoutS when non-zero.\n" +
			"File: go/internal/acssuite/acssuite.go (currently line 237).")
	}
}

func TestC17_121_KbNoEnvGetenv(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	kb := filepath.Join(root, "go", "internal", "research", "kb.go")
	if !acsassert.FileNotContains(t, kb, `os.Getenv("EVOLVE_KB_SEARCH_PATHS")`) {
		t.Errorf("RED: kb.go still calls os.Getenv(\"EVOLVE_KB_SEARCH_PATHS\").\n" +
			"Builder must add PathsConfig.KBSearchPaths to policy.go and update\n" +
			"SearchPathsFromEnv to accept a PathsConfig argument instead.\n" +
			"File: go/internal/research/kb.go (currently line 67).")
	}
}

func TestLookup_AcsGoTimeoutSAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_ACS_GO_TIMEOUT_S")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_ACS_GO_TIMEOUT_S\") returned found=true.\n" +
			"Builder must delete the row from registry_table.go after replacing\n" +
			"envGet(\"EVOLVE_ACS_GO_TIMEOUT_S\") with ACSConfig.GoTimeoutS in acssuite.go.")
	}
}

func TestLookup_KbSearchPathsAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_KB_SEARCH_PATHS")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_KB_SEARCH_PATHS\") returned found=true.\n" +
			"Builder must delete the row from registry_table.go after replacing\n" +
			"os.Getenv(\"EVOLVE_KB_SEARCH_PATHS\") with PathsConfig.KBSearchPaths in kb.go.")
	}
}

func TestC17_124neg_EmptyACSConfigZeroTimeout(t *testing.T) {
	cfg := policy.Policy{}.ACSTimeoutConfig()
	if cfg.GoTimeoutS != 0 {
		t.Errorf("RED: policy.Policy{}.ACSTimeoutConfig().GoTimeoutS = %d; want 0.\n"+
			"An absent ACS policy block must return ACSConfig{GoTimeoutS:0} so\n"+
			"acssuite.goLaneTimeout falls through to DefaultTimeout (60s).\n"+
			"A zero return from the policy accessor must NEVER be passed as a 0-second timeout.",
			cfg.GoTimeoutS)
	}
}

func TestC17_125edge_EmptyPathsConfigKBFallback(t *testing.T) {
	paths := research.SearchPathsFromEnv(policy.PathsConfig{})
	if len(paths) == 0 {
		t.Fatalf("RED: research.SearchPathsFromEnv(PathsConfig{}) returned empty paths.\n" +
			"Empty PathsConfig.KBSearchPaths must fall back to the default:\n" +
			"  knowledge-base/research/:.evolve/instincts/lessons/:docs/research/\n" +
			"Builder must update SearchPathsFromEnv to accept PathsConfig and preserve fallback.")
	}
	for _, p := range paths {
		if strings.Contains(p, "knowledge-base") {
			return
		}
	}
	t.Errorf("RED: fallback paths do not include a 'knowledge-base/' entry.\n"+
		"Got: %v\nExpected at least one path containing 'knowledge-base'.", paths)
}

func TestC17_130_PhaseRootsNoEnvRead(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	catalog := filepath.Join(root, "go", "internal", "phasespec", "mergedcatalog.go")
	if !acsassert.FileNotContains(t, catalog, "os.Getenv(rootsEnv)") {
		t.Errorf("RED: mergedcatalog.go still calls os.Getenv(rootsEnv).\n" +
			"Builder must add:\n" +
			"  func RootsWithPolicy(projectRoot string, cfg policy.PathsConfig) []string\n" +
			"and have Roots(projectRoot string) load policy and delegate.\n" +
			"File: go/internal/phasespec/mergedcatalog.go (currently line 30).")
	}
}

func TestLookup_PhaseRootsAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_PHASE_ROOTS")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_PHASE_ROOTS\") returned found=true.\n" +
			"Builder must delete the row from registry_table.go after removing\n" +
			"os.Getenv(rootsEnv) from mergedcatalog.go and wiring PathsConfig.PhaseRoots.")
	}
}

func TestC17_132neg_AbsentPathsConfigDefaultFallback(t *testing.T) {
	testRoot := t.TempDir()
	roots := phasespec.RootsWithPolicy(testRoot, policy.PathsConfig{})
	if len(roots) == 0 {
		t.Fatalf("RED: phasespec.RootsWithPolicy(root, PathsConfig{}) returned empty slice.\n" +
			"Absent PhaseRoots must fall back to the defaultRoot (.evolve/phases).\n" +
			"Builder must implement:\n" +
			"  func RootsWithPolicy(projectRoot string, cfg policy.PathsConfig) []string\n" +
			"in go/internal/phasespec/mergedcatalog.go.")
	}
	wantSuffix := filepath.Join(".evolve", "phases")
	for _, r := range roots {
		if strings.HasSuffix(r, wantSuffix) {
			return
		}
	}
	t.Errorf("RED: RootsWithPolicy(root, PathsConfig{}) = %v;\n"+
		"none of the returned paths ends with %q.\n"+
		"Absent PhaseRoots must resolve to <projectRoot>/.evolve/phases.", roots, wantSuffix)
}

func TestC17_133edge_AbsolutePathPassThrough(t *testing.T) {
	const absPath = "/absolute/phase/root"
	roots := phasespec.RootsWithPolicy("/some/project", policy.PathsConfig{PhaseRoots: absPath})
	for _, r := range roots {
		if r == absPath {
			return
		}
	}
	t.Errorf("RED: phasespec.RootsWithPolicy(\"/some/project\", PathsConfig{PhaseRoots: %q})\n"+
		"returned %v; want the absolute path %q preserved verbatim (not joined with projectRoot).\n"+
		"Builder must apply the same filepath.IsAbs check that current Roots() uses.", absPath, roots, absPath)
}

func TestC17_999_RegistryCountIs24(t *testing.T) {
	got := len(flagregistry.All)
	if got != 24 {
		t.Errorf("RED: len(flagregistry.All) = %d; want 24 (29 baseline − 5 deletions).\n"+
			"Builder must delete ALL 5 target rows from go/internal/flagregistry/registry_table.go:\n"+
			"  EVOLVE_CLI_MAX_CONCURRENT_CODEX  (dead — 0 literal readers)\n"+
			"  EVOLVE_MODEL_CATALOG_DIR          (wired via BridgePolicy.CatalogDir fn-var)\n"+
			"  EVOLVE_ACS_GO_TIMEOUT_S           (wired via policy.ACSConfig.GoTimeoutS)\n"+
			"  EVOLVE_KB_SEARCH_PATHS            (wired via policy.PathsConfig.KBSearchPaths)\n"+
			"  EVOLVE_PHASE_ROOTS                (wired via policy.PathsConfig.PhaseRoots)",
			got)
	}
}
