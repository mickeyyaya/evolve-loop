//go:build acs

package cycle354

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func controlFlagsPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "control-flags.md")
}

// acs-predicate: config-check
func TestC354_001_CoreInfraFlagsNotActive(t *testing.T) {
	path := controlFlagsPath(t)

	if !acsassert.FileNotContains(t, path, "`EVOLVE_RESOLVE_ROOTS_LOADED` | ACTIVE") {
		t.Errorf("RED: control-flags.md still shows EVOLVE_RESOLVE_ROOTS_LOADED as ACTIVE "+
			"in the hand-maintained cluster table.\n"+
			"Builder must update the Core Infrastructure cluster row to DEAD.\n"+
			"File: %s", path)
	}

	if !acsassert.FileNotContains(t, path, "`EVOLVE_FAILURE_CLASSIFICATIONS_LOADED` | ACTIVE") {
		t.Errorf("RED: control-flags.md still shows EVOLVE_FAILURE_CLASSIFICATIONS_LOADED as ACTIVE "+
			"in the hand-maintained cluster table.\n"+
			"Builder must update the Core Infrastructure cluster row to DEAD.\n"+
			"File: %s", path)
	}
}

// acs-predicate: config-check
func TestC354_002_PlatformHybridFlagsNotActive(t *testing.T) {
	path := controlFlagsPath(t)
	deadPlatformFlags := []string{
		"`EVOLVE_GEMINI_CLAUDE_PATH` | ACTIVE",
		"`EVOLVE_GEMINI_REQUIRE_FULL` | ACTIVE",
		"`EVOLVE_CODEX_CLAUDE_PATH` | ACTIVE",
		"`EVOLVE_ALLOW_INTERACTIVE_FALLBACK` | ACTIVE",
		"`EVOLVE_FORCE_BARE` | ACTIVE",
	}
	for _, pattern := range deadPlatformFlags {
		if !acsassert.FileNotContains(t, path, pattern) {
			t.Errorf("RED: control-flags.md still shows %q as ACTIVE in the Platform/CLI Hybrid cluster.\n"+
				"Builder must update this row to DEAD.\nFile: %s", pattern, path)
		}
	}
}

// acs-predicate: config-check
func TestC354_003_StrictFailuresNotDeprecated(t *testing.T) {
	path := controlFlagsPath(t)
	if !acsassert.FileNotContains(t, path, "`EVOLVE_STRICT_FAILURES` | DEPRECATED") {
		t.Errorf("RED: control-flags.md still shows EVOLVE_STRICT_FAILURES as DEPRECATED "+
			"in the Workflow Defaults cluster.\n"+
			"Builder must update this row to DEAD (registry: StatusDead, no reader).\n"+
			"File: %s", path)
	}
}

func TestC354_005_FlagsCheckExitsZero(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binPath := filepath.Join(root, "go", "bin", "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"bash", "-c", "cd "+root+" && "+binPath+" flags check",
	)
	combined := strings.TrimSpace(out + "\n" + errOut)
	if code != 0 || err != nil {
		t.Errorf("evolve flags check exited %d: %v\nOutput:\n%s\n"+
			"Builder must run `evolve flags generate` after any registry_table.go changes.",
			code, err, combined)
	}
}

func TestC354_006_AuditGofmtGateIsWired(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"./internal/phases/audit/...",
		"-run", "TestNewDefault_WiresGofmtCheck",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/phases/audit/... -run TestNewDefault_WiresGofmtCheck failed (exit=%d).\n"+
			"The gofmt CI-parity gate must be wired in audit.NewDefault.\n"+
			"Fix: audit.go New() must pass CheckGofmt: gofmtCheckDefault in the Config.\n\nOutput:\n%s",
			code, combined)
	}
}

func TestC354_007_AuditSkillsDriftGateIsWired(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"./internal/phases/audit/...",
		"-run", "TestNewDefault_WiresSkillsDriftCheck",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/phases/audit/... -run TestNewDefault_WiresSkillsDriftCheck failed (exit=%d).\n"+
			"The SKILL.md-drift gate must be wired in audit.NewDefault.\n"+
			"Fix: audit.go New() must pass CheckSkillsDrift: skillsDriftCheckDefault in the Config.\n\nOutput:\n%s",
			code, combined)
	}
}
