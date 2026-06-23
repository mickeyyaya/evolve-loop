//go:build acs

// Package cycle17 materializes the cycle-17 acceptance criteria for 4 flag-reduction
// tasks, targeting 5 EVOLVE_* flags for elimination (registry 29 → 24 rows):
//
//   - dead-flag-delete: delete EVOLVE_CLI_MAX_CONCURRENT_CODEX registry row (0 literal readers)
//   - catalog-dir-di: convert EVOLVE_MODEL_CATALOG_DIR to fn-var DI via BridgePolicy.CatalogDir
//   - policy-acs-kb: add ACSConfig+PathsConfig to policy.go; wire EVOLVE_ACS_GO_TIMEOUT_S
//     and EVOLVE_KB_SEARCH_PATHS to these new policy structs
//   - phase-roots-policy: convert EVOLVE_PHASE_ROOTS to PathsConfig.PhaseRoots in policy.json
//
// AC map (1:1 with scout-report.md ACs, all in triage top_n):
//
//	dead-flag-delete:
//	  AC1  Lookup("EVOLVE_CLI_MAX_CONCURRENT_CODEX").found == false           → TestLookup_CliMaxConcurrentCodexAbsent
//	  AC2  No literal in registry_table.go                                    → TestC17_101_CliMaxConcurrentCodexNoLiteralInRegistry
//	  AC3- Driver prefix "EVOLVE_CLI_MAX_CONCURRENT_" still in driver source  → PRE-EXISTING GREEN (currently true; regression guard)
//
//	catalog-dir-di:
//	  AC1  No os.Getenv("EVOLVE_MODEL_CATALOG_DIR") in catalog_overlay.go     → TestC17_110_CatalogDirNoOsGetenv
//	  AC2  No os.Setenv("EVOLVE_MODEL_CATALOG_DIR") in cmd_cycle.go           → TestC17_111_CatalogDirNoOsSetenv
//	  AC3  Lookup("EVOLVE_MODEL_CATALOG_DIR").found == false                  → TestLookup_ModelCatalogDirAbsent
//	  AC4- modelCatalogDirFn fn-var seam present in catalog_overlay.go        → TestC17_113neg_CatalogDirFnVarInPlace
//
//	policy-acs-kb:
//	  AC1  No env read for EVOLVE_ACS_GO_TIMEOUT_S in acssuite.go             → TestC17_120_AcsSuiteNoEnvGetenv
//	  AC2  No env read for EVOLVE_KB_SEARCH_PATHS in kb.go                    → TestC17_121_KbNoEnvGetenv
//	  AC3a Lookup("EVOLVE_ACS_GO_TIMEOUT_S").found == false                   → TestLookup_AcsGoTimeoutSAbsent
//	  AC3b Lookup("EVOLVE_KB_SEARCH_PATHS").found == false                    → TestLookup_KbSearchPathsAbsent
//	  AC5- Empty policy → ACSTimeoutConfig.GoTimeoutS==0 (uses DefaultTimeout) → TestC17_124neg_EmptyACSConfigZeroTimeout
//	  AC6- Empty PathsConfig → KB fallback dirs non-empty                     → TestC17_125edge_EmptyPathsConfigKBFallback
//
//	phase-roots-policy:
//	  AC1  No os.Getenv(rootsEnv) in mergedcatalog.go                         → TestC17_130_PhaseRootsNoEnvRead
//	  AC2  phasespec test suite green                                          → PRE-EXISTING GREEN (suite ok now; guard against regression)
//	  AC4  Lookup("EVOLVE_PHASE_ROOTS").found == false                        → TestLookup_PhaseRootsAbsent
//	  AC5- Absent PathsConfig.PhaseRoots → defaultRoot fallback               → TestC17_132neg_AbsentPathsConfigDefaultFallback
//	  AC6- Absolute path in PhaseRoots passes through unchanged                → TestC17_133edge_AbsolutePathPassThrough
//
//	ALL tasks:
//	  count len(flagregistry.All) == 24 (29 − 5)                              → TestC17_999_RegistryCountIs24
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  AC3-NEG (dynamic driver prefix preserved), AC4-NEG (fn-var seam),
//	           AC5-NEG (nil-safe ACSConfig default), AC5-NEG (absent PathsConfig fallback)
//	Edge/OOD:  AC6-EDGE (absent PathsConfig → KB fallback), AC6-EDGE (absolute path pass-through)
//	Lexical:   Lookup, FileNotContains, FileContains, ACSTimeoutConfig, SearchPathsFromEnv,
//	           RootsWithPolicy, len(All) — seven distinct verbs
//	Semantic:  registry-deletion, env-read-removal, env-write-removal, fn-var-injection,
//	           policy-struct-accessor, nil-safety, absolute-path-preservation
//
// RED state: Package fails to compile because policy.PathsConfig, policy.ACSConfig
// (via ACSTimeoutConfig()), phasespec.RootsWithPolicy, and the new
// research.SearchPathsFromEnv(policy.PathsConfig{}) signature do not exist yet.
// Compile failure = RED (per ACS README: "RED = compile failure or t.Errorf/t.Fatalf").
//
// Pre-existing GREEN: AC3-NEG (driver prefix), AC2 phase-roots (phasespec suite currently passes).
//
// Floor binding (R9.3): predicates authored only for tasks in triage top_n.
// No predicates for deferred or dropped tasks.
package cycle17

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/internal/phasespec"
	"github.com/mickeyyaya/evolveloop/go/internal/policy"
	"github.com/mickeyyaya/evolveloop/go/internal/research"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// ── dead-flag-delete ─────────────────────────────────────────────────────────

// TestLookup_CliMaxConcurrentCodexAbsent verifies that the
// EVOLVE_CLI_MAX_CONCURRENT_CODEX registry row has been deleted.
// The flag has 0 literal readers in Go source; the driver constructs the name
// dynamically at runtime ("EVOLVE_CLI_MAX_CONCURRENT_"+strings.ToUpper(lp.name)).
//
// BEHAVIORAL: calls flagregistry.Lookup directly.
//
// RED: row still in registry_table.go → Lookup returns found=true.
// GREEN: Builder deletes the row → found=false.
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

// TestC17_101_CliMaxConcurrentCodexNoLiteralInRegistry verifies that the literal
// string "EVOLVE_CLI_MAX_CONCURRENT_CODEX" is absent from registry_table.go after
// the row deletion.
//
// // acs-predicate: config-check — registry literal absence is the structural contract.
//
// RED: literal present in registry_table.go (row still exists).
// GREEN: Builder deletes the row.
func TestC17_101_CliMaxConcurrentCodexNoLiteralInRegistry(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	registryFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileNotContains(t, registryFile, "EVOLVE_CLI_MAX_CONCURRENT_CODEX") {
		t.Errorf("RED: registry_table.go still contains 'EVOLVE_CLI_MAX_CONCURRENT_CODEX'.\n" +
			"Builder must delete the row (currently around line 15 of registry_table.go).")
	}
}

// ── catalog-dir-di ───────────────────────────────────────────────────────────

// TestC17_110_CatalogDirNoOsGetenv verifies that catalog_overlay.go no longer calls
// os.Getenv("EVOLVE_MODEL_CATALOG_DIR") after the DI fn-var migration.
//
// // acs-predicate: config-check — env-read absence is the structural contract.
//
// RED: os.Getenv("EVOLVE_MODEL_CATALOG_DIR") present at line 27 of catalog_overlay.go.
// GREEN: Builder replaces with var modelCatalogDirFn reading BridgePolicy.CatalogDir.
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

// TestC17_111_CatalogDirNoOsSetenv verifies that cmd_cycle.go no longer calls
// os.Setenv("EVOLVE_MODEL_CATALOG_DIR", ...) after the DI migration.
//
// // acs-predicate: config-check — env-write absence is the structural contract.
//
// RED: os.Setenv("EVOLVE_MODEL_CATALOG_DIR", ...) present at line 245 of cmd_cycle.go.
// GREEN: Builder replaces with bridge.SetModelCatalogDirFn(evolveDir) or equivalent.
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

// TestLookup_ModelCatalogDirAbsent verifies that the EVOLVE_MODEL_CATALOG_DIR
// registry row has been deleted after the DI fn-var migration.
//
// BEHAVIORAL: calls flagregistry.Lookup directly.
//
// RED: row still in registry_table.go → found=true.
// GREEN: Builder deletes the row after wiring via BridgePolicy.CatalogDir.
func TestLookup_ModelCatalogDirAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_MODEL_CATALOG_DIR")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_MODEL_CATALOG_DIR\") returned found=true.\n" +
			"Builder must delete the EVOLVE_MODEL_CATALOG_DIR row from registry_table.go\n" +
			"after replacing the os.Getenv/os.Setenv pair with fn-var DI.")
	}
}

// TestC17_113neg_CatalogDirFnVarInPlace verifies that catalog_overlay.go declares
// a "modelCatalogDirFn" fn-var (injection seam) rather than a plain func with
// os.Getenv inside.
//
// // acs-predicate: config-check — fn-var presence is the structural injection contract.
//
// RED: "modelCatalogDirFn" absent from catalog_overlay.go (currently: func modelCatalogDir()).
// GREEN: Builder replaces the func with var modelCatalogDirFn and an exported setter.
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

// ── policy-acs-kb ────────────────────────────────────────────────────────────

// TestC17_120_AcsSuiteNoEnvGetenv verifies that acssuite.go no longer calls
// envGet("EVOLVE_ACS_GO_TIMEOUT_S") after the ACSConfig policy migration.
//
// // acs-predicate: config-check — env-read absence is the structural contract.
//
// RED: envGet("EVOLVE_ACS_GO_TIMEOUT_S") present at line 237 of acssuite.go.
// GREEN: Builder reads ACSConfig.GoTimeoutS from policy instead; goLaneTimeout
// uses it when non-zero, falls through to DefaultTimeout when zero.
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

// TestC17_121_KbNoEnvGetenv verifies that research/kb.go no longer calls
// os.Getenv("EVOLVE_KB_SEARCH_PATHS") after the PathsConfig policy migration.
//
// // acs-predicate: config-check — env-read absence is the structural contract.
//
// RED: os.Getenv("EVOLVE_KB_SEARCH_PATHS") present at line 67 of kb.go.
// GREEN: Builder adds PathsConfig{KBSearchPaths string} to policy.go and changes
// SearchPathsFromEnv to accept a PathsConfig parameter.
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

// TestLookup_AcsGoTimeoutSAbsent verifies that the EVOLVE_ACS_GO_TIMEOUT_S
// registry row has been deleted after the ACSConfig migration.
//
// BEHAVIORAL: calls flagregistry.Lookup directly.
//
// RED: row still in registry_table.go → found=true.
// GREEN: Builder deletes the row after wiring via ACSConfig.GoTimeoutS.
func TestLookup_AcsGoTimeoutSAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_ACS_GO_TIMEOUT_S")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_ACS_GO_TIMEOUT_S\") returned found=true.\n" +
			"Builder must delete the row from registry_table.go after replacing\n" +
			"envGet(\"EVOLVE_ACS_GO_TIMEOUT_S\") with ACSConfig.GoTimeoutS in acssuite.go.")
	}
}

// TestLookup_KbSearchPathsAbsent verifies that the EVOLVE_KB_SEARCH_PATHS
// registry row has been deleted after the PathsConfig migration.
//
// BEHAVIORAL: calls flagregistry.Lookup directly.
//
// RED: row still in registry_table.go → found=true.
// GREEN: Builder deletes the row after wiring via PathsConfig.KBSearchPaths.
func TestLookup_KbSearchPathsAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_KB_SEARCH_PATHS")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_KB_SEARCH_PATHS\") returned found=true.\n" +
			"Builder must delete the row from registry_table.go after replacing\n" +
			"os.Getenv(\"EVOLVE_KB_SEARCH_PATHS\") with PathsConfig.KBSearchPaths in kb.go.")
	}
}

// TestC17_124neg_EmptyACSConfigZeroTimeout verifies that policy.Policy{}.ACSTimeoutConfig()
// returns an ACSConfig with GoTimeoutS==0 for an empty policy (nil ACS field).
// A zero GoTimeoutS tells acssuite.goLaneTimeout to fall through to DefaultTimeout (60s),
// so a missing policy block never causes a zero-timeout panic.
//
// BEHAVIORAL: calls policy.Policy{}.ACSTimeoutConfig() — fails to compile before
// Builder adds ACSConfig and ACSTimeoutConfig() to policy.go.
//
// RED: policy.ACSTimeoutConfig / policy.ACSConfig undefined → compile failure.
// GREEN: Builder adds the method; empty policy → GoTimeoutS==0.
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

// TestC17_125edge_EmptyPathsConfigKBFallback verifies that research.SearchPathsFromEnv
// with an empty PathsConfig returns the documented default KB search paths (non-empty).
// This guards against a regression where a nil/absent config causes an empty path list
// that silently breaks KB lookups.
//
// BEHAVIORAL: calls research.SearchPathsFromEnv(policy.PathsConfig{}) — fails to
// compile before Builder changes SearchPathsFromEnv to accept a PathsConfig argument.
//
// RED: research.SearchPathsFromEnv currently takes 0 arguments → compile failure.
// GREEN: Builder updates signature; empty PathsConfig.KBSearchPaths → fallback to
//
//	"knowledge-base/research/:.evolve/instincts/lessons/:docs/research/".
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
			return // found the expected default entry
		}
	}
	t.Errorf("RED: fallback paths do not include a 'knowledge-base/' entry.\n"+
		"Got: %v\nExpected at least one path containing 'knowledge-base'.", paths)
}

// ── phase-roots-policy ───────────────────────────────────────────────────────

// TestC17_130_PhaseRootsNoEnvRead verifies that mergedcatalog.go no longer calls
// os.Getenv(rootsEnv) (where rootsEnv = "EVOLVE_PHASE_ROOTS") after the policy migration.
//
// // acs-predicate: config-check — env-read absence is the structural contract.
//
// RED: os.Getenv(rootsEnv) present at line 30 of mergedcatalog.go.
// GREEN: Builder adds RootsWithPolicy(root, cfg PathsConfig) and updates Roots()
// to load policy and delegate, eliminating the os.Getenv call.
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

// TestLookup_PhaseRootsAbsent verifies that the EVOLVE_PHASE_ROOTS registry row
// has been deleted after the PathsConfig migration.
//
// BEHAVIORAL: calls flagregistry.Lookup directly.
//
// RED: row still in registry_table.go → found=true.
// GREEN: Builder deletes the row after converting mergedcatalog.go:Roots() to
// read policy.PathsConfig.PhaseRoots.
func TestLookup_PhaseRootsAbsent(t *testing.T) {
	_, found := flagregistry.Lookup("EVOLVE_PHASE_ROOTS")
	if found {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_PHASE_ROOTS\") returned found=true.\n" +
			"Builder must delete the row from registry_table.go after removing\n" +
			"os.Getenv(rootsEnv) from mergedcatalog.go and wiring PathsConfig.PhaseRoots.")
	}
}

// TestC17_132neg_AbsentPathsConfigDefaultFallback verifies that RootsWithPolicy
// returns the default ".evolve/phases" root when PathsConfig.PhaseRoots is empty.
// This is the nil-safety contract: an absent policy block must never produce an
// empty roots slice that silently breaks phase discovery.
//
// BEHAVIORAL: calls phasespec.RootsWithPolicy — fails to compile before Builder
// adds the function to mergedcatalog.go.
//
// RED: phasespec.RootsWithPolicy undefined → compile failure.
// GREEN: Builder adds RootsWithPolicy; empty PhaseRoots → ["<root>/.evolve/phases"].
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
			return // found the default root
		}
	}
	t.Errorf("RED: RootsWithPolicy(root, PathsConfig{}) = %v;\n"+
		"none of the returned paths ends with %q.\n"+
		"Absent PhaseRoots must resolve to <projectRoot>/.evolve/phases.", roots, wantSuffix)
}

// TestC17_133edge_AbsolutePathPassThrough verifies that an absolute path in
// PathsConfig.PhaseRoots is returned unchanged (not joined with projectRoot).
// The existing Roots() function already handles absolute paths correctly; this
// predicate ensures RootsWithPolicy preserves that behavior.
//
// BEHAVIORAL: calls phasespec.RootsWithPolicy — fails to compile before Builder
// adds the function.
//
// RED: phasespec.RootsWithPolicy undefined → compile failure.
// GREEN: absolute PhaseRoots → returned verbatim (not joined with projectRoot).
func TestC17_133edge_AbsolutePathPassThrough(t *testing.T) {
	const absPath = "/absolute/phase/root"
	roots := phasespec.RootsWithPolicy("/some/project", policy.PathsConfig{PhaseRoots: absPath})
	for _, r := range roots {
		if r == absPath {
			return // absolute path preserved
		}
	}
	t.Errorf("RED: phasespec.RootsWithPolicy(\"/some/project\", PathsConfig{PhaseRoots: %q})\n"+
		"returned %v; want the absolute path %q preserved verbatim (not joined with projectRoot).\n"+
		"Builder must apply the same filepath.IsAbs check that current Roots() uses.", absPath, roots, absPath)
}

// ── ALL tasks ────────────────────────────────────────────────────────────────

// TestC17_999_RegistryCountIs24 verifies that the flag registry has exactly 24
// rows after all 5 deletions (29 baseline → 24 target).
//
// BEHAVIORAL: len(flagregistry.All) — direct in-process assertion.
//
// RED: len(All) == 29 (all 5 target rows still present).
// GREEN: Builder deletes all 5 rows → len(All) == 24.
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
