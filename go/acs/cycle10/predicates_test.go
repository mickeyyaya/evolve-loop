//go:build acs

// Package cycle10 materializes the cycle-10 acceptance criteria for the
// committed top_n task:
//
//	test-seam-registry-sweep — remove all 65 StatusTestSeam rows from
//	registry_table.go; lower FlagCeiling 241→176; reword EVOLVE_E2E_LIVE
//	step name in go.yml; regenerate control-flags.md.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	test-seam-registry-sweep:
//	  AC3      Registry row count == 176                              → C10_001 (behavioral, exact count)
//	  AC2      FlagCeiling const == 176                              → C10_002 (config-check, waiver)
//	  AC1+NEG1 No StatusTestSeam entries in flagregistry.All         → C10_003 (behavioral, status scan)
//	  NEG1(b)  Representative removed flags absent from Lookup       → C10_004 (behavioral, absence checks)
//	  EDGE1    EVOLVE_E2E_LIVE absent from .github/workflows/go.yml  → C10_005 (config-check, waiver)
//
// ACs with manual+checklist disposition (enforced by CI, no cycle predicate needed):
//
//	AC4  (flagregistry tests pass): TestAll_SortedByName + TestRegistry_FlagCeiling in CI
//	AC5  (full suite 0 FAIL): CI pipeline
//	AC6  (flagreaders guard passes): CI acs lane — go test -tags acs ./acs/regression/flagreaders/...
//	AC8  (registry sorted): TestAll_SortedByName in normal CI run
//
// Removed ACs:
//
//	AC7  (ACS cycle10 predicates pass): self-referential — unverifiable-remove
//	EDGE2 (.apicover-enforce has cycle10): pre-existing GREEN (TDD adds ./acs/cycle10/ during RED phase)
//
// AC9  (no production Go refs to StatusTestSeam outside registry.go):
//
//	Covered by C10_003 — any non-zero StatusTestSeam count in flagregistry.All
//	implies a production registry row references it; plus the flagreaders guard
//	(AC6) catches non-test env reads post-removal. No separate predicate needed.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (test-seam-registry-sweep). Deferred tasks (OBSERVER consolidation, Internal
// batch classification, etc.) get zero predicates this cycle.
package cycle10

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC10_001_RegistryRowCountIs176 verifies that after removing all 65
// StatusTestSeam rows the total registry count is exactly 176.
//
// BEHAVIORAL: calls flagregistry.All directly (the production SSOT slice).
// No source-file grepping; a magic-string patch cannot satisfy this.
// Both over-removal (< 176) and under-removal (> 176) fail the assertion.
//
// RED: len(flagregistry.All) is currently 241, which is 65 rows above 176.
func TestC10_001_RegistryRowCountIs176(t *testing.T) {
	const want = 176
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove all 65 StatusTestSeam rows from registry_table.go.\n"+
			"Both over-removal (< 176) and under-removal (> 176) fail.\n"+
			"Expected: 241 − 65 = 176.",
			got, want)
	}
}

// TestC10_002_FlagCeilingConstIs176 verifies that the FlagCeiling ratchet
// constant in registry_ceiling_test.go has been lowered from 241 to 176
// in the same diff as the row removal.
//
// // acs-predicate: config-check — the constant value is the canonical ratchet
// config; still reading 241 directly breaks the ratchet guarantee post-removal.
//
// RED: registry_ceiling_test.go currently has FlagCeiling = 241.
func TestC10_002_FlagCeilingConstIs176(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	ceilingFile := filepath.Join(root, "go", "internal", "flagregistry", "registry_ceiling_test.go")
	if !acsassert.FileContains(t, ceilingFile, "FlagCeiling = 176") {
		t.Errorf("RED: registry_ceiling_test.go does not contain 'FlagCeiling = 176'.\n"+
			"Builder must lower the FlagCeiling constant from 241 to 176 in the same diff\n"+
			"as removing the 65 StatusTestSeam rows (241 − 65 = 176).\n"+
			"File: %s", ceilingFile)
	}
}

// TestC10_003_NoTestSeamFlagsInRegistry verifies that zero entries in
// flagregistry.All carry StatusTestSeam after Builder removes all 65 rows.
//
// BEHAVIORAL: iterates the live production SSOT slice and checks the Status
// field of every entry. A source-level magic-string patch cannot satisfy this;
// the registry row must be absent for the slice to contain no TestSeam entries.
// Covers AC1 (StatusTestSeam count = 0 in registry_table.go) and AC9 (no
// production registration of TestSeam-status flags).
//
// RED: flagregistry.All currently has 65 entries with Status == StatusTestSeam.
func TestC10_003_NoTestSeamFlagsInRegistry(t *testing.T) {
	var present []string
	for _, f := range flagregistry.All {
		if f.Status == flagregistry.StatusTestSeam {
			present = append(present, f.Name)
		}
	}
	if len(present) > 0 {
		t.Errorf("RED: flagregistry.All has %d StatusTestSeam entries (want 0).\n"+
			"Builder must delete all StatusTestSeam rows from registry_table.go.\n"+
			"Still-registered TestSeam flags (%d): %v",
			len(present), len(present), present)
	}
}

// TestC10_004_RepresentativeTestSeamFlagsAbsentFromLookup verifies that a
// representative cross-cluster sample of the 65 removed flags are no longer
// registered after Builder removes their rows.
//
// Covers NEG1 (removed flags absent from registry). Selects one flag from each
// major TestSeam cluster (per-phase model/CLI/permission/interactive-policy,
// E2E, bridge, observer, other) to provide cross-cluster adversarial coverage.
// A Lookup returning (flag, true) means the row was not removed.
//
// BEHAVIORAL: calls flagregistry.Lookup() for each representative flag —
// the production SSOT function. A source edit alone cannot satisfy this; the
// row must be absent for Lookup to return ok=false.
//
// RED: all 65 TestSeam rows are currently present; each Lookup returns ok=true.
func TestC10_004_RepresentativeTestSeamFlagsAbsentFromLookup(t *testing.T) {
	// One representative from each major TestSeam cluster (scout-report §Research).
	// Lexical diversity: Lookup is the uniform verb; cluster diversity is semantic.
	representatives := []string{
		// Per-phase *_MODEL cluster (9 flags)
		"EVOLVE_AUDITOR_MODEL",
		"EVOLVE_SCOUT_MODEL",
		"EVOLVE_TDD_MODEL",
		// Per-phase *_PERMISSION_MODE cluster (8 flags)
		"EVOLVE_AUDIT_PERMISSION_MODE",
		"EVOLVE_TDD_PERMISSION_MODE",
		// Per-phase *_INTERACTIVE_POLICY cluster (4 flags)
		"EVOLVE_AUDITOR_INTERACTIVE_POLICY",
		// Per-phase *_CLI cluster (2 flags)
		"EVOLVE_BUILD_CLI",
		"EVOLVE_SCOUT_CLI",
		// Per-phase *_PLAN_INPUT/OUTPUT/SYSTEM_PROMPT cluster (3 flags)
		"EVOLVE_BUILD_PLAN_INPUT",
		"EVOLVE_BUILD_PLAN_OUTPUT",
		"EVOLVE_BUILD_SYSTEM_PROMPT",
		// E2E integration seams cluster (11 flags)
		"EVOLVE_E2E_LIVE",
		"EVOLVE_E2E_LIVE_SMOKE",
		"EVOLVE_E2E_LIVE_SOAK",
		// Bridge live/integration seams cluster (4 flags)
		"EVOLVE_BRIDGE_INTEGRATION_LIVE",
		"EVOLVE_BRIDGE_LIVE_CLI",
		// Observer test seam (1 flag)
		"EVOLVE_OBSERVER_TEST_KEY",
		// Other test seams cluster (23 flags)
		"EVOLVE_BASH_PARITY",
		"EVOLVE_NAMELESS",
		"EVOLVE_SCOUT_LATENCY_CEILING_S",
		"EVOLVE_RESEARCH_HOOK_DISABLED",
		"EVOLVE_X",
	}
	for _, name := range representatives {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — TestSeam flag still registered.\n"+
				"Builder must remove this row from registry_table.go (cycle-10 TestSeam sweep).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC10_005_E2ELiveAbsentFromGoYmlStepName verifies that the string
// "EVOLVE_E2E_LIVE" has been removed from the .github/workflows/go.yml
// step name on line 65 (scout-report §Research EVOLVE_E2E_LIVE surface hit).
//
// The original step name — "e2e tier (no race; live sub-tier self-skips without
// EVOLVE_E2E_LIVE)" — contains the flag name as a comment. The flagreaders
// guard's textFlagRE matches it as a non-test surface hit. Fix: reword to
// "without live flag" (preserving meaning; removing the flag reference).
//
// // acs-predicate: config-check — verifies the CI YAML step name does not
// reference a removed flag, keeping the flagreaders guard clean (AC6/H2).
//
// RED: .github/workflows/go.yml:65 currently contains "EVOLVE_E2E_LIVE".
func TestC10_005_E2ELiveAbsentFromGoYmlStepName(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	goYml := filepath.Join(root, ".github", "workflows", "go.yml")
	if !acsassert.FileNotContains(t, goYml, "EVOLVE_E2E_LIVE") {
		t.Errorf("RED: .github/workflows/go.yml still references EVOLVE_E2E_LIVE.\n"+
			"Builder must reword the step name at line 65:\n"+
			"  from: '... live sub-tier self-skips without EVOLVE_E2E_LIVE)'\n"+
			"  to:   '... live sub-tier self-skips without live flag)'\n"+
			"File: %s", goYml)
	}
}
