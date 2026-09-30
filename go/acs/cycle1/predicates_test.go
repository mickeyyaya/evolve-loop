//go:build acs

package cycle1

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1_001_ConcurrentTestFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "bridge", "codex_pretrust_concurrent_test.go")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored and dropped at ship", rel)
	}
}

func TestC1_002_ConcurrentTwoGoroutinesTestPassesWithRace(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=5",
		"-tags", "integration",
		"-run", "TestPretrustCodexProjects_ConcurrentTwoGoroutines",
		"./internal/bridge/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestPretrustCodexProjects_ConcurrentTwoGoroutines") {
		t.Fatalf("RED: TestPretrustCodexProjects_ConcurrentTwoGoroutines did not execute — function missing or name mismatch:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: concurrent two-goroutine pretrust test failed (exit %d):\n%s", code, combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE detected — flock serialization may be broken:\n%s", combined)
	}
}

// acs-predicate: config-check — verifying test structure is inherently a
func TestC1_003_ConcurrentTest_NameAndSeamPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "bridge", "codex_pretrust_concurrent_test.go")
	acsassert.FileMatchesRegex(t, path, `TestPretrustCodexProjects_Concurrent`)
	acsassert.FileContains(t, path, `EVOLVE_CODEX_CONFIG_PATH`)
}

func TestC1_004_ConcurrentTest_NoProductionFilesModified(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, _, code, _ := acsassert.SubprocessOutput(
		"git", "-C", root, "show", "--name-only", "--format=", "HEAD",
		"--", "go/internal/bridge/*.go",
	)
	if code != 0 {
		t.Skip("git show --name-only failed; not in a commit context")
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, "_test.go") {
			continue
		}
		t.Errorf("RED: production file modified in test-only slice: %s", line)
	}
}

func TestC1_005_GofmtGateWiredInAuditNewDefault(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-run", "TestNewDefault_WiresGofmtCheck",
		"-count=1", "-tags", "integration",
		"./internal/phases/audit/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: gofmt gate not wired in audit.NewDefault (TestNewDefault_WiresGofmtCheck FAIL, exit %d):\n%s", code, combined)
	}
}

func TestC1_006_SkillsDriftGateWiredInAuditNewDefault(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-run", "TestNewDefault_WiresSkillsDriftCheck",
		"-count=1", "-tags", "integration",
		"./internal/phases/audit/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: skills-drift gate not wired in audit.NewDefault (TestNewDefault_WiresSkillsDriftCheck FAIL, exit %d):\n%s", code, combined)
	}
}
