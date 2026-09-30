//go:build acs

package cycle2

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC2_001_SessionreaperPackageExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "sessionreaper", "sessionreaper.go")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored and dropped at ship", rel)
	}
}

func TestC2_002_SessionreaperExportsCompile(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build",
		"-C", goDir,
		"./internal/sessionreaper/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: sessionreaper package does not compile (exports may be missing):\n%s", combined)
	}
}

func TestC2_003_FreshLeaseSkippedTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestReapOrphans_FreshLeaseSkipped",
		"./internal/sessionreaper/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestReapOrphans_FreshLeaseSkipped") {
		t.Fatalf("RED: TestReapOrphans_FreshLeaseSkipped did not execute — function missing or name mismatch:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE detected in FreshLeaseSkipped test:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestReapOrphans_FreshLeaseSkipped failed (exit %d):\n%s", code, combined)
	}
}

func TestC2_004_StaleLeaseReapedTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestReapOrphans_StaleLeaseReaped",
		"./internal/sessionreaper/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestReapOrphans_StaleLeaseReaped") {
		t.Fatalf("RED: TestReapOrphans_StaleLeaseReaped did not execute — function missing or name mismatch:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE in StaleLeaseReaped test:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestReapOrphans_StaleLeaseReaped failed (exit %d):\n%s", code, combined)
	}
}

func TestC2_005_MissingRegistryIsZeroActivityTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestReapOrphans_MissingRegistryIsZeroActivity",
		"./internal/sessionreaper/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestReapOrphans_MissingRegistryIsZeroActivity") {
		t.Fatalf("RED: TestReapOrphans_MissingRegistryIsZeroActivity did not execute:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestReapOrphans_MissingRegistryIsZeroActivity failed (exit %d):\n%s", code, combined)
	}
}

func TestC2_006_AbsentLeaseIsStaleTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestReapOrphans_AbsentLeaseIsStale",
		"./internal/sessionreaper/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestReapOrphans_AbsentLeaseIsStale") {
		t.Fatalf("RED: TestReapOrphans_AbsentLeaseIsStale did not execute:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestReapOrphans_AbsentLeaseIsStale failed (exit %d):\n%s", code, combined)
	}
}

// acs-predicate: config-check — source-wiring assertion is inherently a
func TestC2_007_LooppreflightGlobWarnRemovedAndReapOrphansWired(t *testing.T) {
	root := acsassert.RepoRoot(t)
	checksPath := filepath.Join(root, "go", "internal", "looppreflight", "checks.go")
	acsassert.FileNotContains(t, checksPath, "stale bridge tmux session(s)")
	acsassert.FileMatchesRegex(t, checksPath, `ReapOrphans`)
}

func TestC2_008_LooppreflightTestsPassAfterReapOrphansWiring(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-count=1", "-tags", "integration",
		"./internal/looppreflight/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: looppreflight integration tests failed (exit %d):\n%s", code, combined)
	}
}

func TestC2_009_SwarmReapOrphansDryRunSucceeds(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "run",
		"-C", goDir,
		"./cmd/evolve/...",
		"swarm", "reap-orphans", "--dry-run",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: `evolve swarm reap-orphans --dry-run` failed (exit %d):\n%s", code, combined)
	}
}

// acs-predicate: config-check — enrollment verification is inherently a
func TestC2_010_ApiCoverEnforceContainsSessionreaper(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforcePath := filepath.Join(root, "go", ".apicover-enforce")
	// acs-predicate: config-check
	acsassert.FileContains(t, enforcePath, "./internal/sessionreaper")
}

func TestC2_011_ApiCoverEnforceTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1", "-tags", "acs",
		"-run", "TestApicoverEnforce_CoversEveryInternalPackage",
		"./acs/regression/apicover/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestApicoverEnforce_CoversEveryInternalPackage") {
		t.Fatalf("RED: TestApicoverEnforce_CoversEveryInternalPackage did not execute:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestApicoverEnforce_CoversEveryInternalPackage failed (exit %d):\n%s", code, combined)
	}
}

func TestC2_012_SessionreaperCoverageAtLeast85Pct(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	coverFile := filepath.Join(t.TempDir(), "sessionreaper.cover.out")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-count=1", "-tags", "integration",
		"-coverprofile", coverFile,
		"-coverpkg", "./internal/sessionreaper/...",
		"./internal/sessionreaper/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: sessionreaper integration tests failed (exit %d):\n%s", code, combined)
	}
	for _, line := range strings.Split(combined, "\n") {
		if strings.Contains(line, "coverage:") && strings.Contains(line, "%") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "coverage:" && i+1 < len(fields) {
					pctStr := strings.TrimSuffix(fields[i+1], "%")
					var pct float64
					if _, scanErr := fmt.Sscanf(pctStr, "%f", &pct); scanErr == nil {
						if pct < 85.0 {
							t.Errorf("RED: sessionreaper coverage %.1f%% < 85%% threshold", pct)
						}
						return
					}
				}
			}
		}
	}
	t.Errorf("RED: could not parse coverage percentage from output:\n%s", combined)
}
