//go:build acs

package cycle5

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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

// acs-predicate: config-check — ADR structural assertions are inherently
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

// acs-predicate: config-check — runtime-reference.md is an ops documentation
func TestC5_003_RuntimeReferenceHasAllConcurrencyFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rtRef := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	// acs-predicate: config-check
	acsassert.FileContains(t, rtRef, "EVOLVE_LANE")
	acsassert.FileContains(t, rtRef, "EVOLVE_REAP_ORPHANS")
	acsassert.FileContains(t, rtRef, "EVOLVE_CLI_MAX_CONCURRENT")
}

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

func TestC5_007_SessionreaperDoesNotGateOnEnvVar(t *testing.T) {
	root := acsassert.RepoRoot(t)
	reaperPath := filepath.Join(root, "go", "internal", "sessionreaper", "sessionreaper.go")
	acsassert.FileNotContains(t, reaperPath, "EVOLVE_REAP_ORPHANS")
}

// acs-predicate: config-check — source-code absence of the env read confirms
func TestC5_010_HangClassifierEnvReadRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	classifyGo := filepath.Join(root, "go", "internal", "cycleclassify", "classify.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, classifyGo, `"EVOLVE_HANG_CLASSIFIER"`)
}

// acs-predicate: config-check — registry_table.go is the SSOT for flag status.
func TestC5_011_HangClassifierRegistryDeprecated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regTable := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(regTable, "EVOLVE_HANG_CLASSIFIER", "StatusDeprecated") {
		t.Errorf("RED: no line in registry_table.go contains both EVOLVE_HANG_CLASSIFIER and StatusDeprecated — Builder must update the entry")
	}
}

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

// acs-predicate: config-check — source absence confirms const + Getenv deleted.
func TestC5_014_CatalogAutoRefreshEnvReadRemoved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmdModelsLive := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, cmdModelsLive, "EVOLVE_MODELCATALOG_AUTOREFRESH")
}

// acs-predicate: config-check
func TestC5_015_CatalogAutoRefreshRegistryDeprecated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regTable := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(regTable, "EVOLVE_MODELCATALOG_AUTOREFRESH", "StatusDeprecated") {
		t.Errorf("RED: no line in registry_table.go contains both EVOLVE_MODELCATALOG_AUTOREFRESH and StatusDeprecated — Builder must update the entry")
	}
}

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

// acs-predicate: config-check — source param-name absence confirms the rename.
func TestC5_018_ShouldRefreshCatalogParamIsAutoRefreshBool(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmdModelsLive := filepath.Join(root, "go", "cmd", "evolve", "cmd_models_live.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, cmdModelsLive, "disableEnvVal")
}

// acs-predicate: config-check
func TestC5_019_AnthropicBaseURLEvolveRemovedFromDriver(t *testing.T) {
	root := acsassert.RepoRoot(t)
	driverPath := filepath.Join(root, "go", "internal", "bridge", "driver_claudetmux.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, driverPath, "EVOLVE_ANTHROPIC_BASE_URL")
}

// acs-predicate: config-check
func TestC5_020_AnthropicBaseURLEvolveRemovedFromSetup(t *testing.T) {
	root := acsassert.RepoRoot(t)
	setupPath := filepath.Join(root, "go", "internal", "setup", "setup.go")
	// acs-predicate: config-check
	acsassert.FileNotContains(t, setupPath, "EVOLVE_ANTHROPIC_BASE_URL")
}

// acs-predicate: config-check
func TestC5_021_AnthropicBaseURLRegistryDeprecated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	regTable := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	// acs-predicate: config-check
	if !acsassert.LineContainsAll(regTable, "EVOLVE_ANTHROPIC_BASE_URL", "StatusDeprecated") {
		t.Errorf("RED: no line in registry_table.go contains both EVOLVE_ANTHROPIC_BASE_URL and StatusDeprecated — Builder must update the entry")
	}
}

// acs-predicate: config-check — struct field presence in policy.go SSOT.
func TestC5_022_BridgePolicyHasAnthropicBaseURLField(t *testing.T) {
	root := acsassert.RepoRoot(t)
	policyGo := filepath.Join(root, "go", "internal", "policy", "policy.go")
	// acs-predicate: config-check
	acsassert.FileContains(t, policyGo, "AnthropicBaseURL")
}

func TestC5_023_BridgeConfigTestCoversAnthropicBaseURLAndPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	bridgeTestFile := filepath.Join(root, "go", "internal", "policy", "bridge_config_param_test.go")
	// acs-predicate: config-check (auxiliary — confirms new test cases added)
	acsassert.FileContains(t, bridgeTestFile, "AnthropicBaseURL")
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

func TestC5_024_RawAnthropicBaseURLPreservedInDriver(t *testing.T) {
	root := acsassert.RepoRoot(t)
	driverPath := filepath.Join(root, "go", "internal", "bridge", "driver_claudetmux.go")
	acsassert.FileContains(t, driverPath, `"ANTHROPIC_BASE_URL"`)
}
