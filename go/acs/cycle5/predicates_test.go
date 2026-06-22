//go:build acs

// Package cycle5 materializes the cycle-5 acceptance criteria for:
//
//   - concurrent-loop-adr-docs: Slice 6 of the concurrency-arch-slices campaign.
//     Deliverables: ADR-0054 (sibling-worktree architecture doc), runtime-reference
//     flag entries for EVOLVE_LANE/EVOLVE_REAP_ORPHANS/EVOLVE_CLI_MAX_CONCURRENT_<CLI>,
//     and flag registry rows for EVOLVE_REAP_ORPHANS + EVOLVE_CLI_MAX_CONCURRENT_<CLI>.
//
//   - convert-hang-classifier-to-policy: EVOLVE_HANG_CLASSIFIER → ClassifyPolicy
//     struct + ClassifyConfig() accessor in policy.go; os.Getenv removed from
//     classify.go; registry entry → StatusDeprecated; apicover test added.
//
//   - convert-modelcatalog-autorefresh-to-policy: EVOLVE_MODELCATALOG_AUTOREFRESH
//     → CatalogPolicy{AutoRefresh *bool} + CatalogConfig() in policy.go; const +
//     os.Getenv removed from cmd_models_live.go; shouldRefreshCatalog param →
//     bool; registry entry → StatusDeprecated; apicover test added.
//
//   - convert-anthropic-base-url-to-bridge-policy: EVOLVE_ANTHROPIC_BASE_URL →
//     BridgePolicy.AnthropicBaseURL string field; EVOLVE_-prefixed reads removed
//     from driver_claudetmux.go and setup.go; raw ANTHROPIC_BASE_URL reads
//     preserved; registry entry → StatusDeprecated; bridge test extended.
package cycle5

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// --- Task: concurrent-loop-adr-docs ---

// TestC5_001_ADRFileExistsAndTracked asserts that
// docs/architecture/adr/0054-concurrent-evolve-loop-sibling-worktrees.md
// was created in the worktree and is git-tracked. A gitignored file is
// silently dropped at ship (cycle-93 lesson). Also covers AC7 — the ADR
// number must be exactly 0054 (not a renumbered copy of 0053 or 0055).
func TestC5_001_ADRFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk — Builder must create ADR-0054", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored and dropped at ship", rel)
	}
}

// TestC5_002_ADRFileHasRequiredSections verifies that the ADR file contains
// all five required structural elements: a Status section, Layer 1, Layer 2,
// runscope, and a reference to ADR-0049. Encodes AC1's content requirements.
//
// acs-predicate: config-check — ADR structural assertions are inherently
// doc-section-presence checks; the behavioral anchor is TestC5_001 (git-tracked)
// and TestC5_005 (go build passes with the doc committed).
func TestC5_002_ADRFileHasRequiredSections(t *testing.T) {
	root := acsassert.RepoRoot(t)
	adrPath := filepath.Join(root, "docs", "architecture", "adr", "0054-concurrent-evolve-loop-sibling-worktrees.md")
	// acs-predicate: config-check
	acsassert.FileContains(t, adrPath, "## Status")
	acsassert.FileContains(t, adrPath, "Layer 1")
	acsassert.FileContains(t, adrPath, "Layer 2")
	acsassert.FileContains(t, adrPath, "runscope")
	acsassert.FileContains(t, adrPath, "ADR-0049")
}

// TestC5_003_RuntimeReferenceHasAllConcurrencyFlags verifies that
// docs/operations/runtime-reference.md documents all three concurrency flags
// from the sibling-worktree architecture. AC2.
//
// acs-predicate: config-check — runtime-reference.md is an ops documentation
// table; presence of the flag names is the acceptance criterion itself.
func TestC5_003_RuntimeReferenceHasAllConcurrencyFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rtRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	// acs-predicate: config-check
	acsassert.FileContains(t, rtRef, "EVOLVE_LANE")
	acsassert.FileContains(t, rtRef, "EVOLVE_REAP_ORPHANS")
	acsassert.FileContains(t, rtRef, "EVOLVE_CLI_MAX_CONCURRENT")
}

// TestC5_005_GoBuildPassesAfterFlagRows verifies that adding the two flag
// registry rows introduces no compilation regression. AC4.
//
// Pre-existing GREEN expected: the branch is documentation-only (Slice 6).
// HEAD aaf12fc5 passes go build; new flag rows are purely data (no new Go code).
func TestC5_005_GoBuildPassesAfterFlagRows(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build",
		"-C", goDir,
		"./...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: go build ./... failed (exit %d):\n%s", code, combined)
	}
}

// TestC5_006_FlagRegistryTestsPassAfterNewRows verifies that the flagregistry
// unit tests still pass after the two new rows are added. AC5. The flag
// registry package has tests that enforce table invariants; new rows must
// satisfy them.
//
// Pre-existing GREEN expected: flagregistry tests pass on HEAD aaf12fc5.
func TestC5_006_FlagRegistryTestsPassAfterNewRows(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-count=1",
		"./internal/flagregistry/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: go test ./internal/flagregistry/ failed (exit %d):\n%s", code, combined)
	}
}

// TestC5_007_SessionreaperDoesNotGateOnEnvVar verifies that sessionreaper.go
// does NOT read EVOLVE_REAP_ORPHANS to gate its core reap logic. AC6 (negative).
//
// The sessionreaper is unconditionally wired in looppreflight (Slice 3); the
// flag exists solely as operator documentation (an opt-out surface for the
// ops table), not as a runtime code gate. Introducing a conditional os.Getenv
// guard in sessionreaper.go would violate the Slice-3 architecture decision.
//
// Pre-existing GREEN expected: sessionreaper.go currently has no such gate.
func TestC5_007_SessionreaperDoesNotGateOnEnvVar(t *testing.T) {
	root := acsassert.RepoRoot(t)
	reaperPath := filepath.Join(root, "go", "internal", "sessionreaper", "sessionreaper.go")
	acsassert.FileNotContains(t, reaperPath, "EVOLVE_REAP_ORPHANS")
}

// =============================================================================
// Flag-Reduction Campaign Cycle 5: Three EVOLVE_ env reads → policy.json typed
// fields. Tasks: convert-hang-classifier-to-policy (010-013),
// convert-modelcatalog-autorefresh-to-policy (014-018),
// convert-anthropic-base-url-to-bridge-policy (019-024).
// =============================================================================

// --- Task 1: EVOLVE_HANG_CLASSIFIER → policy.ClassifyPolicy ---

// TestC5_010_HangClassifierEnvReadRemoved asserts that classify.go no longer
// reads EVOLVE_HANG_CLASSIFIER from the environment (C1).
//
// acs-predicate: config-check — source-code absence of the env read confirms
// the migration; behavioral coverage is provided by TestC5_013.
func TestC5_010_HangClassifierEnvReadRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	classifyGo := filepath.Join(root, "go", "internal", "cycleclassify", "classify.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, classifyGo, `"EVOLVE_HANG_CLASSIFIER"`)
}

// TestC5_011_HangClassifierRegistryDeprecated asserts that the flagregistry
// entry for EVOLVE_HANG_CLASSIFIER is now StatusDeprecated (C4). The entry
// must appear on a single line together with StatusDeprecated (registry_table.go
// uses one-line struct literals for each flag).
//
// acs-predicate: config-check — registry_table.go is the SSOT for flag status.
func TestC5_011_HangClassifierRegistryDeprecated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regTable := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(regTable, "EVOLVE_HANG_CLASSIFIER", "StatusDeprecated") {
		t.Errorf("RED: no line in registry_table.go contains both EVOLVE_HANG_CLASSIFIER and StatusDeprecated — Builder must update the entry")
	}
}

// TestC5_012_ClassifyConfigTestFileTracked asserts that the new apicover test
// file for ClassifyPolicy/ClassifyConfig was created and is git-tracked (C5).
// A git-untracked file is silently dropped at ship (cycle-93 lesson).
func TestC5_012_ClassifyConfigTestFileTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "policy", "classify_config_param_test.go")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk — Builder must create it", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored (dropped at ship)", rel)
	}
}

// TestC5_013_ClassifyPolicyTestRunsAndPasses asserts that the policy package
// contains TestClassifyConfig* tests and they all pass (C2 + C5 behavioral).
//
// Runs `go test -v -run TestClassifyConfig ./internal/policy/` and checks
// that at least one test ran (=== RUN in stdout) and none failed (exit 0).
// RED when the test file does not exist OR when ClassifyPolicy/ClassifyConfig
// fail to compile.
func TestC5_013_ClassifyPolicyTestRunsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-v", "-count=1",
		"-run", "TestClassifyConfig",
		"./internal/policy/",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: go test -run TestClassifyConfig ./internal/policy/ failed (exit %d):\n%s", code, combined)
	}
	if !strings.Contains(stdout, "=== RUN") {
		t.Errorf("RED: no TestClassifyConfig* tests matched in ./internal/policy/ — classify_config_param_test.go must define TestClassifyConfig_Resolution (and related cases)")
	}
}

// --- Task 2: EVOLVE_MODELCATALOG_AUTOREFRESH → policy.CatalogPolicy ---

// TestC5_014_CatalogAutoRefreshEnvReadRemoved asserts that cmd_models_live.go
// no longer references EVOLVE_MODELCATALOG_AUTOREFRESH (C1). The const
// autoRefreshDisableEnv and the os.Getenv call are both removed.
//
// acs-predicate: config-check — source absence confirms const + Getenv deleted.
func TestC5_014_CatalogAutoRefreshEnvReadRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmdModelsLive := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, cmdModelsLive, "EVOLVE_MODELCATALOG_AUTOREFRESH")
}

// TestC5_015_CatalogAutoRefreshRegistryDeprecated asserts that the registry
// entry for EVOLVE_MODELCATALOG_AUTOREFRESH is now StatusDeprecated (C4).
//
// acs-predicate: config-check
func TestC5_015_CatalogAutoRefreshRegistryDeprecated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regTable := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(regTable, "EVOLVE_MODELCATALOG_AUTOREFRESH", "StatusDeprecated") {
		t.Errorf("RED: no line in registry_table.go contains both EVOLVE_MODELCATALOG_AUTOREFRESH and StatusDeprecated — Builder must update the entry")
	}
}

// TestC5_016_CatalogConfigTestFileTracked asserts that catalog_config_param_test.go
// was created and is git-tracked (C5). Same cycle-93 ship-guard rationale as
// TestC5_012.
func TestC5_016_CatalogConfigTestFileTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "policy", "catalog_config_param_test.go")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk — Builder must create it", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored (dropped at ship)", rel)
	}
}

// TestC5_017_CatalogPolicyTestRunsAndPasses asserts that the policy package
// contains TestCatalogConfig* tests and they all pass (C2 + C5 + C7 behavioral).
//
// The default AutoRefresh=true semantics (C7) must be covered by the test suite
// (an absent CatalogPolicy block must resolve AutoRefresh to true). This
// predicate verifies that the suite exists and passes; C7 specifics live in
// the Builder-written test cases.
func TestC5_017_CatalogPolicyTestRunsAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-v", "-count=1",
		"-run", "TestCatalogConfig",
		"./internal/policy/",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: go test -run TestCatalogConfig ./internal/policy/ failed (exit %d):\n%s", code, combined)
	}
	if !strings.Contains(stdout, "=== RUN") {
		t.Errorf("RED: no TestCatalogConfig* tests matched in ./internal/policy/ — catalog_config_param_test.go must define TestCatalogConfig_Resolution (and related cases)")
	}
}

// TestC5_018_ShouldRefreshCatalogParamIsAutoRefreshBool asserts that the
// shouldRefreshCatalog function no longer has a disableEnvVal string parameter
// (C3 — API contract change). The old param name confirms the old string-based
// API; its absence confirms the rename to autoRefresh bool.
//
// acs-predicate: config-check — source param-name absence confirms the rename.
func TestC5_018_ShouldRefreshCatalogParamIsAutoRefreshBool(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmdModelsLive := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, cmdModelsLive, "disableEnvVal")
}

// --- Task 3: EVOLVE_ANTHROPIC_BASE_URL → BridgePolicy.AnthropicBaseURL ---

// TestC5_019_AnthropicBaseURLEvolveRemovedFromDriver asserts that
// driver_claudetmux.go no longer reads EVOLVE_ANTHROPIC_BASE_URL (C1).
// The raw "ANTHROPIC_BASE_URL" read (no EVOLVE_ prefix) at line 34 is
// preserved — this predicate targets the EVOLVE_-prefixed variant only.
//
// acs-predicate: config-check
func TestC5_019_AnthropicBaseURLEvolveRemovedFromDriver(t *testing.T) {
	root := acsassert.RepoRoot(t)
	driverPath := filepath.Join(root, "go", "internal", "bridge", "driver_claudetmux.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, driverPath, "EVOLVE_ANTHROPIC_BASE_URL")
}

// TestC5_020_AnthropicBaseURLEvolveRemovedFromSetup asserts that setup.go no
// longer reads EVOLVE_ANTHROPIC_BASE_URL (C2). The raw ANTHROPIC_BASE_URL read
// (without EVOLVE_ prefix) in authMode() is preserved for proxy detection.
//
// acs-predicate: config-check
func TestC5_020_AnthropicBaseURLEvolveRemovedFromSetup(t *testing.T) {
	root := acsassert.RepoRoot(t)
	setupPath := filepath.Join(root, "go", "internal", "setup", "setup.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, setupPath, "EVOLVE_ANTHROPIC_BASE_URL")
}

// TestC5_021_AnthropicBaseURLRegistryDeprecated asserts that the registry
// entry for EVOLVE_ANTHROPIC_BASE_URL is now StatusDeprecated (C4).
//
// acs-predicate: config-check
func TestC5_021_AnthropicBaseURLRegistryDeprecated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regTable := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(regTable, "EVOLVE_ANTHROPIC_BASE_URL", "StatusDeprecated") {
		t.Errorf("RED: no line in registry_table.go contains both EVOLVE_ANTHROPIC_BASE_URL and StatusDeprecated — Builder must update the entry")
	}
}

// TestC5_022_BridgePolicyHasAnthropicBaseURLField asserts that BridgePolicy in
// policy.go has the new AnthropicBaseURL string field (C3). This is the struct
// declaration; the behavioral consequence is exercised by TestC5_023.
//
// acs-predicate: config-check — struct field presence in policy.go SSOT.
func TestC5_022_BridgePolicyHasAnthropicBaseURLField(t *testing.T) {
	root := acsassert.RepoRoot(t)
	policyGo := filepath.Join(root, "go", "internal", "policy", "policy.go")
	// acs-predicate: config-check
	acsassert.FileContains(t, policyGo, "AnthropicBaseURL")
}

// TestC5_023_BridgeConfigTestCoversAnthropicBaseURLAndPasses asserts that
// bridge_config_param_test.go exercises the new AnthropicBaseURL field (C5)
// and that the full bridge policy test suite passes (behavioral).
//
// The FileContains check is RED because bridge_config_param_test.go currently
// has no AnthropicBaseURL test cases. The subprocess check is pre-existing
// GREEN (TestBridgeConfig_Resolution already passes) but confirms that adding
// the new field does not break existing tests.
func TestC5_023_BridgeConfigTestCoversAnthropicBaseURLAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	bridgeTestFile := filepath.Join(root, "go", "internal", "policy", "bridge_config_param_test.go")
	// acs-predicate: config-check (auxiliary — confirms new test cases added)
	acsassert.FileContains(t, bridgeTestFile, "AnthropicBaseURL")
	// Behavioral: run the bridge policy tests and confirm they pass.
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-v", "-count=1",
		"-run", "TestBridgeConfig",
		"./internal/policy/",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: go test -run TestBridgeConfig ./internal/policy/ failed (exit %d):\n%s", code, combined)
	}
	if !strings.Contains(stdout, "=== RUN") {
		t.Errorf("RED: no TestBridgeConfig* tests ran in ./internal/policy/ — unexpected (pre-existing tests should run)")
	}
}

// TestC5_024_RawAnthropicBaseURLPreservedInDriver asserts that the raw
// ANTHROPIC_BASE_URL read (without EVOLVE_ prefix) is still present in
// driver_claudetmux.go after removing the EVOLVE_-prefixed read (C6).
//
// PRE-EXISTING GREEN: this read exists at line 34 before Builder touches it
// and must remain after migration (it is the 3rd-party Anthropic env var,
// not the EVOLVE_-namespaced proxy-mode override).
func TestC5_024_RawAnthropicBaseURLPreservedInDriver(t *testing.T) {
	root := acsassert.RepoRoot(t)
	driverPath := filepath.Join(root, "go", "internal", "bridge", "driver_claudetmux.go")
	acsassert.FileContains(t, driverPath, `"ANTHROPIC_BASE_URL"`)
}
