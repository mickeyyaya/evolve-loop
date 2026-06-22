//go:build acs

// Package cycle4 materializes the cycle-4 acceptance criteria for:
//   - Task 1: migrate-di-seams-advisor-workspace
//   - Task 2: migrate-cli-flags-policy-platform-marketplace
package cycle4

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC4_001_NoAdvisorDepthEnvInPhaseAdvisor asserts that EVOLVE_ADVISOR_DEPTH
// is no longer read from the environment in go/internal/core/phase_advisor.go.
func TestC4_001_NoAdvisorDepthEnvInPhaseAdvisor(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "core", "phase_advisor.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileNotContains(t, path, `"EVOLVE_ADVISOR_DEPTH"`)
}

// TestC4_002_NoDisableWorkspaceGuardEnvInCycleRun asserts that EVOLVE_DISABLE_WORKSPACE_GUARD
// is no longer read from the environment in go/internal/core/cyclerun.go.
func TestC4_002_NoDisableWorkspaceGuardEnvInCycleRun(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "core", "cyclerun.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileNotContains(t, path, `"EVOLVE_DISABLE_WORKSPACE_GUARD"`)
}

// TestC4_003_NoPolicyBypassEnvInRunner asserts that EVOLVE_POLICY_BYPASS
// is no longer read from the environment in go/internal/phases/runner/runner.go.
func TestC4_003_NoPolicyBypassEnvInRunner(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileNotContains(t, path, `"EVOLVE_POLICY_BYPASS"`)
}

// TestC4_004_NoPlatformEnvInDetectCli asserts that EVOLVE_PLATFORM
// is no longer read from the environment in go/internal/detectcli/detectcli.go.
func TestC4_004_NoPlatformEnvInDetectCli(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "detectcli", "detectcli.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileNotContains(t, path, `"EVOLVE_PLATFORM"`)
}

// TestC4_005_NoMarketplaceDirEnvInReleasePipelineAndCli asserts that EVOLVE_MARKETPLACE_DIR
// is no longer read from the environment in go/internal/releasepipeline/bridges.go
// and go/internal/cli/opscmd/marketplace_poll.go.
func TestC4_005_NoMarketplaceDirEnvInReleasePipelineAndCli(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path1 := filepath.Join(root, "go", "internal", "releasepipeline", "bridges.go")
	if !acsassert.FileExists(t, path1) {
		t.Fatalf("file missing: %s", path1)
	}
	acsassert.FileNotContains(t, path1, `"EVOLVE_MARKETPLACE_DIR"`)

	path2 := filepath.Join(root, "go", "internal", "cli", "opscmd", "marketplace_poll.go")
	if !acsassert.FileExists(t, path2) {
		t.Fatalf("file missing: %s", path2)
	}
	acsassert.FileNotContains(t, path2, `"EVOLVE_MARKETPLACE_DIR"`)
}

// TestC4_006_FlagsAreDeprecatedInRegistry asserts that all 5 migrated flags
// have had their status updated to StatusDeprecated in registry_table.go.
func TestC4_006_FlagsAreDeprecatedInRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileContains(t, path, `Name: "EVOLVE_ADVISOR_DEPTH", Status: StatusDeprecated`)
	acsassert.FileContains(t, path, `Name: "EVOLVE_DISABLE_WORKSPACE_GUARD", Status: StatusDeprecated`)
	acsassert.FileContains(t, path, `Name: "EVOLVE_POLICY_BYPASS", Status: StatusDeprecated`)
	acsassert.FileContains(t, path, `Name: "EVOLVE_PLATFORM", Status: StatusDeprecated`)
	acsassert.FileContains(t, path, `Name: "EVOLVE_MARKETPLACE_DIR", Status: StatusDeprecated`)
}
