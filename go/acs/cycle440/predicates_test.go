//go:build acs

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

func TestC440_002_DuplicatedBridgeHelperRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bridgeDir := filepath.Join(root, "go", "internal", "bridge")
	stdout, _, code, _ := acsassert.SubprocessOutput("grep", "-rn", "func baseCLIName", bridgeDir)
	if code == 0 {
		t.Errorf("C440_002: func baseCLIName still present in internal/bridge/ (must be consolidated into policy.BaseCLI):\n%s", stdout)
	}
}

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

func TestC440_004_ApicoverNamingFloor(t *testing.T) {
	_, stderr, code := runGoTest(t, "", false, routerPkg, policyPkg, bridgePkg)
	if code != 0 {
		t.Errorf("C440_004: router+policy+bridge suite exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC440_005_GenuineCatalogMissStillClamps(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestClampPlanModelRouting_ClampsCatalogMiss", false, routerPkg)
	if code != 0 {
		t.Errorf("C440_005: catalog-miss-still-clamps test exit=%d\nstderr=%s", code, stderr)
	}
}

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

func TestC440_007_AdvisoryLogsNotApplies(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestModelRouting_AdvisoryLogsNotApplies", false, corePkg)
	if code != 0 {
		t.Errorf("C440_007: advisory-logs-not-applies test exit=%d\nstderr=%s", code, stderr)
	}
}

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

func TestC440_009_NilPlanDegradesToProfileStatic(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestModelRouting_AutoDegradesToProfileStatic", false, corePkg)
	if code != 0 {
		t.Errorf("C440_009: auto-degrades-to-profile-static test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC440_010_BenchedOverlayPrimaryFallsBack(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestRunner_ModelRoutingAuto_BenchedOverlayPrimaryFallsBack", false, runnerPkg)
	if code != 0 {
		t.Errorf("C440_010: benched-overlay-primary-falls-back test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC440_011_NoRegressionAcrossTouchedPackages(t *testing.T) {
	_, stderr, code := runGoTest(t, "", true, corePkg, runnerPkg, routerPkg, llmroutePkg, policyPkg, bridgePkg)
	if code != 0 {
		t.Errorf("C440_011: full -race suite across touched packages exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC440_012_CheckedInRegistryDeclaresAuto(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regPath := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	acsassert.FileMatchesRegex(t, regPath, `"model_routing"\s*:\s*"auto"`)
}

func TestC440_013_GoZeroValueStaysStatic(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestParseModelRouting_ZeroValueStatic", false, configPkg)
	if code != 0 {
		t.Errorf("C440_013: zero-value-stays-static test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC440_014_CheckedInRegistryLoadsAsAuto(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestCheckedInPolicyDefaultsModelRoutingAuto", false, configPkg)
	if code != 0 {
		t.Errorf("C440_014: checked-in-registry-loads-as-auto test exit=%d\nstderr=%s", code, stderr)
	}
}

func TestC440_015_EscapeHatchHonored(t *testing.T) {
	_, stderr, code := runGoTest(t, "TestParseModelRouting_EscapeHatchStaticOff", false, configPkg)
	if code != 0 {
		t.Errorf("C440_015: escape-hatch-honored test exit=%d\nstderr=%s", code, stderr)
	}
}
