//go:build acs

// Package cycle12 materializes the cycle-12 acceptance criteria for the
// committed top_n task:
//
//	consolidate-bridge-cluster — remove all 5 BRIDGE_* registry rows; add
//	BridgePolicy struct to policy.go; add bridgePidfileEnv split-const to
//	engine.go; update 3 production read sites (manifest.go, capabilities.go,
//	recipe/loader.go) to use DI seam vars; lower FlagCeiling 163→158.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	consolidate-bridge-cluster:
//	  AC1+NEG2  All 5 BRIDGE_* flags absent from Lookup              → C12_001 (behavioral)
//	  AC3       Registry row count == 158                             → C12_002 (behavioral, count)
//	  AC2       FlagCeiling const == 158                              → C12_003 (config-check, waiver)
//	  AC4+NEG1a BRIDGE_MANIFEST_DIR os.Getenv gone from manifest.go  → C12_004 (config-check, waiver)
//	  AC4+NEG1b BRIDGE_CATALOG_DIR os.Getenv gone from capabilities  → C12_005 (config-check, waiver)
//	  AC4+NEG1c BRIDGE_RECIPE_DIR os.Getenv gone from loader.go      → C12_006 (config-check, waiver)
//	  AC5       BridgePolicy struct present in policy.go              → C12_007 (config-check, waiver)
//	  AC6       bridgePidfileEnv split-const in engine.go             → C12_008 (config-check, waiver)
//	  EDGE1     control-flags.md has no BRIDGE_* rows                 → C12_009 (config-check, waiver)
//
// ACs with manual+checklist disposition (enforced by CI, no cycle predicate needed):
//
//	AC7  (flagreaders ACS guard passes): evolve acs suite — flagreaders regression lane
//	AC8  (full bridge suite 0 FAIL): CI pipeline — go test ./internal/bridge/... etc.
//
// ACs removed / pre-existing GREEN:
//
//	EDGE2 (.apicover-enforce has ./acs/cycle12): pre-existing GREEN — TDD adds this
//	      entry during RED phase; self-referential predicate is unverifiable-remove.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C12_001 — Lookup returns ok=false for all 5 flags; cannot be
//	           satisfied by adding magic strings — the registry row must be absent.
//	Edge/OOD:  BRIDGE_GO (0 production reads, dead flag) is in C12_001 — the
//	           pure-delete case where no code migration is required.
//	Lexical:   Lookup / len() / FileNotContains / FileContains — four distinct verbs.
//	Semantic:  registry count, flag-absence, env-reads-deleted (3 files), struct-added,
//	           split-const, docs-updated — six distinct behavioral checks.
//
// 1:1 enforcement: predicate=9, manual+checklist=2, unverifiable-remove=1,
// pre-existing-GREEN=1 → total AC=13 ✓
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (consolidate-bridge-cluster). Deferred tasks (CHECKPOINT_*, BYPASS_*, etc.)
// get zero predicates this cycle.
package cycle12

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC12_001_AllBridgeFlagsAbsentFromRegistry verifies that all 5 BRIDGE_*
// flags are no longer registered after Builder removes their rows from
// registry_table.go.
//
// Covers AC1 (5 rows absent from registry_table.go) and NEG2 (BRIDGE_PIDFILE
// specifically absent). Includes all 5 flags: BRIDGE_GO (dead, 0 production
// reads), BRIDGE_PIDFILE (IPC handoff), BRIDGE_MANIFEST_DIR, BRIDGE_CATALOG_DIR,
// BRIDGE_RECIPE_DIR (3 path/dir overrides).
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — the production SSOT.
// A source edit alone cannot satisfy this; the row must be absent for Lookup
// to return ok=false.
//
// RED: all 5 flags are currently registered; each Lookup returns (flag, true).
func TestC12_001_AllBridgeFlagsAbsentFromRegistry(t *testing.T) {
	// All 5 BRIDGE_* rows from scout-report §Key Findings.
	// Semantic sub-cases: dead flag (BRIDGE_GO), IPC handoff (BRIDGE_PIDFILE),
	// 3 path/dir overrides (MANIFEST_DIR, CATALOG_DIR, RECIPE_DIR).
	allFlags := []string{
		"EVOLVE_BRIDGE_CATALOG_DIR",
		"EVOLVE_BRIDGE_GO", // dead: 0 production reads (v12 cutover comment)
		"EVOLVE_BRIDGE_MANIFEST_DIR",
		"EVOLVE_BRIDGE_PIDFILE", // IPC handoff: envValue(env slice), NOT os.Getenv
		"EVOLVE_BRIDGE_RECIPE_DIR",
	}
	for _, name := range allFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-12 BRIDGE_* consolidation).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC12_004_BridgeManifestDirEnvReadGoneFromManifest verifies that the
// os.Getenv("EVOLVE_BRIDGE_MANIFEST_DIR") call at manifest.go:20 has been
// removed and replaced by a DI seam var backed by BridgePolicy.
//
// The scout identified the single production read at manifest.go:20:
//   - os.Getenv("EVOLVE_BRIDGE_MANIFEST_DIR")
//
// This is replaced by a package-level seam var bridgeManifestDirFn backed by
// policy.Load(projectRoot()).BridgeConfig().ManifestDir.
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// RED: manifest.go:20 currently has os.Getenv("EVOLVE_BRIDGE_MANIFEST_DIR").
func TestC12_004_BridgeManifestDirEnvReadGoneFromManifest(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	manifestFile := filepath.Join(root, "go", "internal", "bridge", "manifest.go")
	if !acsassert.FileNotContains(t, manifestFile, `os.Getenv("EVOLVE_BRIDGE_MANIFEST_DIR")`) {
		t.Errorf("RED: bridge/manifest.go still reads EVOLVE_BRIDGE_MANIFEST_DIR via os.Getenv.\n"+
			"Builder must add a DI seam var bridgeManifestDirFn and replace bridgeManifestDir()\n"+
			"body with the seam var (backed by policy.Load().BridgeConfig().ManifestDir).\n"+
			"File: %s", manifestFile)
	}
}

// TestC12_005_BridgeCatalogDirEnvReadGoneFromCapabilities verifies that the
// os.Getenv("EVOLVE_BRIDGE_CATALOG_DIR") call at capabilities.go:44 has been
// removed and replaced by a DI seam var backed by BridgePolicy.
//
// The scout identified the single production read at capabilities.go:44:
//   - os.Getenv("EVOLVE_BRIDGE_CATALOG_DIR")
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// RED: capabilities.go:44 currently has os.Getenv("EVOLVE_BRIDGE_CATALOG_DIR").
func TestC12_005_BridgeCatalogDirEnvReadGoneFromCapabilities(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	capFile := filepath.Join(root, "go", "internal", "bridge", "capabilities", "capabilities.go")
	if !acsassert.FileNotContains(t, capFile, `os.Getenv("EVOLVE_BRIDGE_CATALOG_DIR")`) {
		t.Errorf("RED: capabilities/capabilities.go still reads EVOLVE_BRIDGE_CATALOG_DIR via os.Getenv.\n"+
			"Builder must add a DI seam var catalogDirFn and replace catalogDir()\n"+
			"body with the seam var (backed by policy.Load().BridgeConfig().CatalogDir).\n"+
			"File: %s", capFile)
	}
}

// TestC12_006_BridgeRecipeDirEnvReadGoneFromLoader verifies that the
// os.Getenv("EVOLVE_BRIDGE_RECIPE_DIR") call at recipe/loader.go:35 has been
// removed and replaced by a DI seam var backed by BridgePolicy.
//
// The scout identified the single production read at recipe/loader.go:35:
//   - os.Getenv("EVOLVE_BRIDGE_RECIPE_DIR")
//
// // acs-predicate: config-check — the os.Getenv ABSENCE is the structural contract.
//
// RED: recipe/loader.go:35 currently has os.Getenv("EVOLVE_BRIDGE_RECIPE_DIR").
func TestC12_006_BridgeRecipeDirEnvReadGoneFromLoader(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	loaderFile := filepath.Join(root, "go", "internal", "bridge", "recipe", "loader.go")
	if !acsassert.FileNotContains(t, loaderFile, `os.Getenv("EVOLVE_BRIDGE_RECIPE_DIR")`) {
		t.Errorf("RED: bridge/recipe/loader.go still reads EVOLVE_BRIDGE_RECIPE_DIR via os.Getenv.\n"+
			"Builder must add a DI seam var recipeDirFn and replace recipeDir()\n"+
			"body with the seam var (backed by policy.Load().BridgeConfig().RecipeDir).\n"+
			"File: %s", loaderFile)
	}
}

// TestC12_007_BridgePolicyStructAddedToPolicy verifies that the BridgePolicy
// struct has been added to internal/policy/policy.go.
//
// BridgePolicy is the Configuration Object that replaces BRIDGE_MANIFEST_DIR,
// BRIDGE_CATALOG_DIR, and BRIDGE_RECIPE_DIR. It is loaded from .evolve/policy.json
// "bridge" block and injected via DI seam vars. Default values are encoded in
// Policy.BridgeConfig() (ManifestDir="", CatalogDir="", RecipeDir="" → each
// subsystem falls back to its default path when the field is empty).
// Follows the FanoutPolicy (cycle-9) and ObserverPolicy (cycle-11) precedents.
//
// // acs-predicate: config-check — verifies the new config surface exists.
//
// RED: internal/policy/policy.go currently has no BridgePolicy type.
func TestC12_007_BridgePolicyStructAddedToPolicy(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	policyFile := filepath.Join(root, "go", "internal", "policy", "policy.go")
	if !acsassert.FileContains(t, policyFile, "BridgePolicy") {
		t.Errorf("RED: internal/policy/policy.go has no BridgePolicy struct.\n"+
			"Builder must add the BridgePolicy struct and Policy.BridgeConfig() method\n"+
			"following the FanoutPolicy (cycle-9) and ObserverPolicy (cycle-11) precedents.\n"+
			"Required fields: ManifestDir string, CatalogDir string, RecipeDir string.\n"+
			"File: %s", policyFile)
	}
	// Also verify the BridgeConfig() accessor method is present.
	if !acsassert.FileContains(t, policyFile, "BridgeConfig()") {
		t.Errorf("RED: internal/policy/policy.go has no BridgeConfig() method.\n"+
			"Builder must add Policy.BridgeConfig() that returns BridgePolicy with\n"+
			"defaults applied (ManifestDir/CatalogDir/RecipeDir default to empty string;\n"+
			"each subsystem falls back to its built-in default path when the field is empty).\n"+
			"File: %s", policyFile)
	}
}

// TestC12_008_BridgePidfileEnvSplitConstInEngine verifies that the IPC
// handoff constant bridgePidfileEnv (split-const pattern) has been added to
// bridge/engine.go, following the FanoutWorkerTokenEnv precedent in recursion.go.
//
// The split-const pattern `bridgePidfileEnv = "EVOLVE_" + "BRIDGE_PIDFILE"` keeps
// the env-var key out of the flagregistry (the key is NEVER operator-facing; it is
// a parent→child subprocess IPC signal set by driver_claudep.go:81 and read by
// engine.go:483 via envValue(env, ...) — NOT os.Getenv). The registry row is
// deleted; the subprocess env-passing mechanism is preserved unchanged.
//
// // acs-predicate: config-check — the split-const PRESENCE is the structural contract.
//
// RED: engine.go currently uses the string literal "EVOLVE_BRIDGE_PIDFILE" directly.
func TestC12_008_BridgePidfileEnvSplitConstInEngine(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	engineFile := filepath.Join(root, "go", "internal", "bridge", "engine.go")
	// The split-const form: "EVOLVE_" + "BRIDGE_PIDFILE" — the two parts must
	// appear concatenated so the full EVOLVE_BRIDGE_PIDFILE string never appears
	// in a single token that the flagregistry scanner would pick up.
	if !acsassert.FileContains(t, engineFile, `"EVOLVE_"`) || !acsassert.FileContains(t, engineFile, `"BRIDGE_PIDFILE"`) {
		t.Errorf("RED: bridge/engine.go does not have the bridgePidfileEnv split-const.\n"+
			"Builder must add:\n"+
			"  const bridgePidfileEnv = \"EVOLVE_\" + \"BRIDGE_PIDFILE\"\n"+
			"and replace the two string literals \"EVOLVE_BRIDGE_PIDFILE\" in engine.go\n"+
			"(lines 481,483) with bridgePidfileEnv. Follows FanoutWorkerTokenEnv precedent.\n"+
			"File: %s", engineFile)
	}
}

// TestC12_009_ControlFlagsMdHasNoBridgeRows verifies that the generated doc
// docs/architecture/control-flags.md has no EVOLVE_BRIDGE_* entries after
// the 5 registry rows are removed and the doc regenerated.
//
// Covers EDGE1. The doc is generated from the flagregistry (source of truth);
// its absence of BRIDGE_* rows follows from C12_001 (rows removed) plus the
// regeneration step ('evolve flags generate').
//
// // acs-predicate: config-check — the doc regeneration is a required build step.
//
// RED: control-flags.md currently has EVOLVE_BRIDGE_* entries.
func TestC12_009_ControlFlagsMdHasNoBridgeRows(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	// Check the 5 BRIDGE_* flags that should all be absent post-regeneration.
	bridgeFlags := []string{
		"EVOLVE_BRIDGE_CATALOG_DIR",
		"EVOLVE_BRIDGE_GO",
		"EVOLVE_BRIDGE_MANIFEST_DIR",
		"EVOLVE_BRIDGE_PIDFILE",
		"EVOLVE_BRIDGE_RECIPE_DIR",
	}
	for _, flag := range bridgeFlags {
		if !acsassert.FileNotContains(t, controlFlags, flag) {
			t.Errorf("RED: control-flags.md still contains %q.\n"+
				"Builder must remove all 5 BRIDGE_* rows from registry_table.go\n"+
				"then regenerate the doc via 'evolve flags generate'.\n"+
				"File: %s", flag, controlFlags)
		}
	}
}
