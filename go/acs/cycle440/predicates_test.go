//go:build acs

// Package cycle440 materialises the cycle-440 acceptance criteria for
// model_routing MR4 + MR5 (goal: b17855662370871400e2d044ab4c104dc7d19c940d872b63892330b0bca98049).
//
// Three ## top_n tasks, strictly sequenced A → B → C (triage-report.md):
//
//	Task A (mr4b-basecli-single-source, S): consolidate policy.baseCLI +
//	  bridge.baseCLIName into ONE exported policy.BaseCLI, and normalize the
//	  CLI base name before router.ClampPlanModelRouting's catalog lookup
//	  (F2/F3 — a suffixed "claude-tmux" currently misses the catalog, which
//	  is keyed on the base family "claude").
//	Task B (mr4-projection-apply-degrade, M/L): wire the UNWIRED
//	  router.ClampPlanModelRouting into the cycle-start plan path, thread a
//	  phase→{cli,tier} soft-overlay proposal into PhaseRequest, apply it in
//	  phases/runner/runner.go with pin==nil (soft, not absolute), and degrade
//	  to profile-static whenever no plan (or no per-phase proposal) exists.
//	Task C (mr4d-default-model-routing-auto, S): once B lands, default
//	  model_routing to auto in the checked-in registry (config-driven, not a
//	  Go literal); static/off remain the escape hatch.
//
// AC map (1:1, R9.3 floor-binding; predicates for the ## top_n tasks only):
//
//	Task A AC1 suffixed CLI honored via base name (positive)     → C440_001
//	Task A AC2 single exported source, dup removed (negative)    → C440_002 + C440_003
//	Task A AC3 apicover naming floor (regression)                 → C440_004
//	Task A AC4 genuine catalog miss still clamps (edge/OOD)       → C440_005 (pre-existing GREEN pin)
//	Task B AC1 auto applies a clamped overlay (positive)          → C440_006
//	Task B AC2 advisory logs, does not apply (mode semantics)     → C440_007
//	Task B AC3 static is a noop (regression floor)                → C440_008
//	Task B AC4 nil-plan degrades to profile-static (negative/OOD) → C440_009
//	Task B AC5 benched overlay primary still falls back (edge)    → C440_010
//	Task B AC6 no regression across touched packages              → C440_011
//	Task C AC1 checked-in registry declares model_routing=auto    → C440_012
//	Task C AC2 Go zero-value stays static (regression floor)      → C440_013
//	Task C AC3 checked-in registry loads as auto (config-default) → C440_014
//	Task C AC4 escape hatch (static/off) honored (negative)       → C440_015
//
// 1:1 enforcement: predicate=15 → total AC = 14 (Task A AC4 gets two
// predicates: the pre-existing-GREEN pin plus its share of C440_004's
// compile-and-pass regression sweep) ✓ every AC ≥1 predicate, none double-
// counted as a DIFFERENT AC.
//
// *** IMPORTANT DISCREPANCY — Task C's target file (read this before auditing
// C440_012/C440_014) ***
//
// The scout report, api-contract.md, and eval mr4d-default-model-routing-
// auto.md all say the default flip belongs in ".evolve/policy.json". Reading
// the ACTUAL producer (go/internal/config/config.go's registryDoc, whose
// model_routing field is bound to registryPath's `config.model_routing` key)
// and every real call site that builds registryPath (cmd/evolve/cmd_cycle.go,
// internal/cli/phasecmd/phase_verify.go, internal/router/policy.go) shows
// model_routing is parsed EXCLUSIVELY from
// docs/architecture/phase-registry.json. .evolve/policy.json is a SEPARATE
// file (policy.Load) that feeds Policy.Pins/MandatoryPhases/ShipFloor/etc. —
// it never reaches RoutingConfig.ModelRouting. config.go's own comments
// (lines 294/598/600) are stale/wrong on this point (a pre-existing
// documentation bug, out of this cycle's scope to fix elsewhere). C440_012 /
// C440_014 target the file the code actually reads (Rule 8: read first, don't
// invent an API from context) rather than the eval's literal (incorrect)
// grep target. See internal/config/model_routing_default_test.go's doc
// comment on TestCheckedInPolicyDefaultsModelRoutingAuto for the full trail.
// Builder/Auditor: edit docs/architecture/phase-registry.json's `config`
// object, NOT .evolve/policy.json, to satisfy Task C.
//
// *** Task A landmine for Builder: an existing test calls the doomed helper
// directly ***
//
// internal/bridge/catalog_overlay_test.go:78 calls the unexported
// baseCLIName(...) directly. AC2 requires that helper GONE — Builder must
// update/remove that assertion (route it through policy.BaseCLI, mirroring
// applyCatalogTierMap's own call-site fix) as part of the Task A removal, not
// leave it as a dangling compile error.
//
// RED strategy: C440_001/004/006-011 are compile-fail RED today (they
// reference policy.BaseCLI, llmroute.Overlay/ApplySoftOverlay,
// core.PhaseRequest.ModelRoutingCLI/Tier, and core.WithModelCatalogLookup —
// none exist yet on main). C440_002/003 are grep-based structural RED (the
// duplication still exists). C440_012/014 are behaviorally RED (the checked-
// in registry has no model_routing key yet, so it parses to static, not
// auto). C440_005/013/015 are documented pre-existing GREEN pins (the
// underlying behavior is already correct and unaffected by this cycle's
// change — see doc comments on the referenced tests).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C440_002 (duplication genuinely removed, not merely papered
//	            over — a source-region scan defeats a redundant helper, not
//	            just a passing value check), C440_009 (advisor outage must not
//	            silently apply a stale/garbage overlay), C440_015 (escape
//	            hatch must still work once the default flips)
//	Edge/OOD:   C440_005 (a genuine catalog miss, not just a suffix mismatch),
//	            C440_010 (benched overlay primary — health-state edge case)
//	Semantic:   C440_007 (advisory RECORDS the proposal but does not DISPATCH
//	            it — a distinct property from "computes the right value")
package cycle440

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	policyPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/policy"
	routerPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/router"
	bridgePkg   = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	llmroutePkg = "github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	corePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/core"
	runnerPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	configPkg   = "github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func runGoTest(t *testing.T, runFilter string, race bool, pkgs ...string) (string, string, int) {
	t.Helper()
	args := []string{"test", "-count=1"}
	if race {
		args = append(args, "-race")
	}
	if runFilter != "" {
		args = append(args, "-run", runFilter)
	}
	args = append(args, pkgs...)
	stdout, stderr, code, _ := acsassert.SubprocessOutput("go", args...)
	return stdout, stderr, code
}

// TestC440_001_SuffixedCLIHonoredViaBaseName (Task A AC1, positive).
func TestC440_001_SuffixedCLIHonoredViaBaseName(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestBaseCLI_StripsKnownDriverSuffixes|TestBaseCLI_UnrecognizedSuffixUnchanged", false, policyPkg)
	if code != 0 {
		t.Errorf("C440_001a: policy.BaseCLI tests exit=%d\nstderr=%s", code, stderr)
	}
	_, stderr, code = runGoTest(t, "TestClampPlanModelRouting_SuffixedCLIHonoredViaBaseName", false, routerPkg)
	if code != 0 {
		t.Errorf("C440_001b: suffixed-CLI clamp test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_002_DuplicatedBridgeHelperRemoved (Task A AC2, negative,
// discriminating): the unexported bridge.baseCLIName duplicate must be gone.
// RED today: it still exists at catalog_overlay.go:116.
func TestC440_002_DuplicatedBridgeHelperRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bridgeDir := filepath.Join(root, "go", "internal", "bridge")
	stdout, _, code, _ := acsassert.SubprocessOutput("grep", "-rn", "func baseCLIName", bridgeDir)
	if code == 0 {
		t.Errorf("C440_002: func baseCLIName still present in internal/bridge/ (must be consolidated into policy.BaseCLI):\n%s", stdout)
	}
}

// TestC440_003_SingleExportedBaseNameSource (Task A AC2, negative,
// discriminating): exactly ONE file in internal/ defines the exported
// base-name helper. RED today: zero matches (the exported symbol doesn't
// exist yet).
func TestC440_003_SingleExportedBaseNameSource(t *testing.T) {
	root := acsassert.RepoRoot(t)
	internalDir := filepath.Join(root, "go", "internal")
	stdout, _, code, _ := acsassert.SubprocessOutput("grep", "-rl",
		"func BaseCLI\\|func BaseName\\|func BaseCLIName", internalDir)
	if code != 0 {
		t.Fatalf("C440_003: no file defines an exported base-name helper (grep exit=%d)", code)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if strings.TrimSpace(line) != "" {
			files = append(files, line)
		}
	}
	if len(files) != 1 {
		t.Errorf("C440_003: expected exactly 1 file defining the exported base-name helper, got %d: %v", len(files), files)
	}
}

// TestC440_004_ApicoverNamingFloor (Task A AC3, regression): router + policy
// + bridge must compile and pass with the new exported symbol referenced in
// a _test.go (apicover -enforce naming floor, ADR-0069 CI-parity).
func TestC440_004_ApicoverNamingFloor(t *testing.T) {
	_, stderr, code := runGoTest(t, "", false, routerPkg, policyPkg, bridgePkg)
	if code != 0 {
		t.Errorf("C440_004: router+policy+bridge suite exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_005_GenuineCatalogMissStillClamps (Task A AC4, edge/OOD,
// pre-existing GREEN pin): a genuine catalog miss (no suffix involved) must
// still clamp — the normalization must not swallow real misses.
func TestC440_005_GenuineCatalogMissStillClamps(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestClampPlanModelRouting_ClampsCatalogMiss", false, routerPkg)
	if code != 0 {
		t.Errorf("C440_005: catalog-miss-still-clamps test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_006_AutoAppliesClampedOverlay (Task B AC1, positive).
func TestC440_006_AutoAppliesClampedOverlay(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestModelRouting_AutoApplies|TestModelRouting_CatalogMissClampsUnderAuto|TestPhaseRequest_ModelRoutingFieldsOmitEmptyByDefault", false, corePkg)
	if code != 0 {
		t.Errorf("C440_006a: core auto-applies tests exit=%d\nstderr=%s", code, stderr)
	}
	_, stderr, code = runGoTest(t, "TestRunner_ModelRoutingAuto_SoftOverlayAppliesAsPrimary", false, runnerPkg)
	if code != 0 {
		t.Errorf("C440_006b: runner soft-overlay-primary test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_007_AdvisoryLogsNotApplies (Task B AC2, semantic/mode-gating).
func TestC440_007_AdvisoryLogsNotApplies(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestModelRouting_AdvisoryLogsNotApplies", false, corePkg)
	if code != 0 {
		t.Errorf("C440_007: advisory-logs-not-applies test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_008_StaticIsNoop (Task B AC3, regression floor).
func TestC440_008_StaticIsNoop(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestModelRouting_StaticIsNoop", false, corePkg)
	if code != 0 {
		t.Errorf("C440_008a: core static-noop test exit=%d\nstderr=%s", code, stderr)
	}
	_, stderr, code = runGoTest(t, "TestRunner_ModelRoutingAuto_ZeroOverlayByteIdentical", false, runnerPkg)
	if code != 0 {
		t.Errorf("C440_008b: runner zero-overlay-byte-identical test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_009_NilPlanDegradesToProfileStatic (Task B AC4, negative/OOD).
func TestC440_009_NilPlanDegradesToProfileStatic(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestModelRouting_AutoDegradesToProfileStatic", false, corePkg)
	if code != 0 {
		t.Errorf("C440_009: auto-degrades-to-profile-static test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_010_BenchedOverlayPrimaryFallsBack (Task B AC5, edge: benched
// health state).
func TestC440_010_BenchedOverlayPrimaryFallsBack(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestRunner_ModelRoutingAuto_BenchedOverlayPrimaryFallsBack", false, runnerPkg)
	if code != 0 {
		t.Errorf("C440_010: benched-overlay-primary-falls-back test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_011_NoRegressionAcrossTouchedPackages (Task B AC6, regression,
// race-checked).
func TestC440_011_NoRegressionAcrossTouchedPackages(t *testing.T) {
	_, stderr, code := runGoTest(t, "", true, corePkg, runnerPkg, routerPkg, llmroutePkg, policyPkg, bridgePkg)
	if code != 0 {
		t.Errorf("C440_011: full -race suite across touched packages exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_012_CheckedInRegistryDeclaresAuto (Task C AC1, positive). Targets
// docs/architecture/phase-registry.json — see the package-doc discrepancy
// note; this is the file config.Load actually reads, NOT .evolve/policy.json
// as the eval's literal grep command claims.
func TestC440_012_CheckedInRegistryDeclaresAuto(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regPath := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	acsassert.FileMatchesRegex(t, regPath, `"model_routing"\s*:\s*"auto"`)
}

// TestC440_013_GoZeroValueStaysStatic (Task C AC2, regression floor,
// pre-existing GREEN pin).
func TestC440_013_GoZeroValueStaysStatic(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestParseModelRouting_ZeroValueStatic", false, configPkg)
	if code != 0 {
		t.Errorf("C440_013: zero-value-stays-static test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_014_CheckedInRegistryLoadsAsAuto (Task C AC3, config-default).
func TestC440_014_CheckedInRegistryLoadsAsAuto(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestCheckedInPolicyDefaultsModelRoutingAuto", false, configPkg)
	if code != 0 {
		t.Errorf("C440_014: checked-in-registry-loads-as-auto test exit=%d\nstderr=%s", code, stderr)
	}
}

// TestC440_015_EscapeHatchHonored (Task C AC4, negative, pre-existing GREEN
// pin).
func TestC440_015_EscapeHatchHonored(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestParseModelRouting_EscapeHatchStaticOff", false, configPkg)
	if code != 0 {
		t.Errorf("C440_015: escape-hatch-honored test exit=%d\nstderr=%s", code, stderr)
	}
}
