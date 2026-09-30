//go:build acs

package cycle3

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC3_001_CliadmitPackageExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "cliadmit", "cliadmit.go")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored and dropped at ship", rel)
	}
}

func TestC3_002_CliadmitPackageCompiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "build",
		"-C", goDir,
		"./internal/cliadmit/...",
	)
	combined := stdout + "\n" + stderr
	if code != 0 {
		t.Fatalf("RED: cliadmit package does not compile (Acquire/release exports may be missing):\n%s", combined)
	}
}

func TestC3_003_AcquireUnboundedTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestAcquire_Unbounded",
		"./internal/cliadmit/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestAcquire_Unbounded") {
		t.Fatalf("RED: TestAcquire_Unbounded did not execute — function missing or name mismatch:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE detected in TestAcquire_Unbounded:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestAcquire_Unbounded failed (exit %d):\n%s", code, combined)
	}
}

func TestC3_004_AcquireMaxOneTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestAcquire_MaxOne",
		"./internal/cliadmit/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestAcquire_MaxOne") {
		t.Fatalf("RED: TestAcquire_MaxOne did not execute — function missing or name mismatch:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE in TestAcquire_MaxOne:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestAcquire_MaxOne failed (exit %d):\n%s", code, combined)
	}
}

func TestC3_005_AcquireMutualExclusionTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestAcquire_MutualExclusion",
		"./internal/cliadmit/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestAcquire_MutualExclusion") {
		t.Fatalf("RED: TestAcquire_MutualExclusion did not execute — function missing or name mismatch:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE in TestAcquire_MutualExclusion:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestAcquire_MutualExclusion failed (exit %d):\n%s", code, combined)
	}
}

func TestC3_006_AcquireStaleHolderPrunedTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestAcquire_StaleHolderPruned",
		"./internal/cliadmit/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestAcquire_StaleHolderPruned") {
		t.Fatalf("RED: TestAcquire_StaleHolderPruned did not execute — function missing or name mismatch:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE in TestAcquire_StaleHolderPruned:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestAcquire_StaleHolderPruned failed (exit %d):\n%s", code, combined)
	}
}

func TestC3_007_AcquireReleaseFreesSlotTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestAcquire_ReleaseFreesSlot",
		"./internal/cliadmit/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestAcquire_ReleaseFreesSlot") {
		t.Fatalf("RED: TestAcquire_ReleaseFreesSlot did not execute — function missing or name mismatch:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE in TestAcquire_ReleaseFreesSlot:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestAcquire_ReleaseFreesSlot failed (exit %d):\n%s", code, combined)
	}
}

func TestC3_008_AcquireContextCancelUnblocksTestPasses(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=1",
		"-tags", "integration",
		"-run", "TestAcquire_ContextCancelUnblocks",
		"./internal/cliadmit/...",
	)
	combined := stdout + "\n" + stderr
	if !strings.Contains(combined, "TestAcquire_ContextCancelUnblocks") {
		t.Fatalf("RED: TestAcquire_ContextCancelUnblocks did not execute — function missing or name mismatch:\n%s", combined)
	}
	if strings.Contains(combined, "DATA RACE") {
		t.Errorf("RED: DATA RACE in TestAcquire_ContextCancelUnblocks:\n%s", combined)
	}
	if code != 0 {
		t.Fatalf("RED: TestAcquire_ContextCancelUnblocks failed (exit %d):\n%s", code, combined)
	}
}

// acs-predicate: config-check — source-wiring assertion is inherently a
func TestC3_009_CliadmitHookedInDriverTmuxRepl(t *testing.T) {
	root := acsassert.RepoRoot(t)
	driverPath := filepath.Join(root, "go", "internal", "bridge", "driver_tmux_repl.go")
	if !acsassert.FileContains(t, driverPath, "cliadmit") {
		t.Errorf("RED: cliadmit not imported or referenced in driver_tmux_repl.go")
	}
	if !acsassert.FileContains(t, driverPath, "cliadmit.Acquire") {
		t.Errorf("RED: cliadmit.Acquire not called in driver_tmux_repl.go")
	}
	if !acsassert.FileContains(t, driverPath, "EVOLVE_CLI_MAX_CONCURRENT_") {
		t.Errorf("RED: EVOLVE_CLI_MAX_CONCURRENT_ dial not referenced in driver_tmux_repl.go")
	}
}

// acs-predicate: config-check — enrollment verification is inherently a
func TestC3_010_ApiCoverEnforceContainsCliadmit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforcePath := filepath.Join(root, "go", ".apicover-enforce")
	// acs-predicate: config-check
	acsassert.FileContains(t, enforcePath, "./internal/cliadmit")
}

func TestC3_011_ApiCoverEnforceTestPasses(t *testing.T) {
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
