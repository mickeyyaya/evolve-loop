//go:build acs

package cycle4

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC4_001_NoAdvisorDepthEnvInPhaseAdvisor(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "core", "phase_advisor.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileNotContains(t, path, `"EVOLVE_ADVISOR_DEPTH"`)
}

func TestC4_002_NoDisableWorkspaceGuardEnvInCycleRun(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "core", "cyclerun.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileNotContains(t, path, `"EVOLVE_DISABLE_WORKSPACE_GUARD"`)
}

func TestC4_003_NoPolicyBypassEnvInRunner(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "phases", "runner", "runner.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileNotContains(t, path, `"EVOLVE_POLICY_BYPASS"`)
}

func TestC4_004_NoPlatformEnvInDetectCli(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "detectcli", "detectcli.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileNotContains(t, path, `"EVOLVE_PLATFORM"`)
}

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

func TestC4_006_FlagsAreDeprecatedInRegistry(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	if !acsassert.FileExists(t, path) {
		t.Fatalf("file missing: %s", path)
	}
	acsassert.FileContains(t, path, `Name: "EVOLVE_ADVISOR_DEPTH", Status: StatusDeprecated`)
	acsassert.FileContains(t, path, `Name: "EVOLVE_DISABLE_WORKSPACE_GUARD", Status: StatusDeprecated`)
	acsassert.FileContains(t, path, `Name: "EVOLVE_PLATFORM", Status: StatusDeprecated`)
	acsassert.FileContains(t, path, `Name: "EVOLVE_MARKETPLACE_DIR", Status: StatusDeprecated`)
}
