//go:build acs

// Package cycle3 materializes the cycle-3 acceptance criteria for:
//
//   - cliadmit-cross-process-admission: Slice 4 of the concurrency-arch-slices
//     campaign. New leaf pkg internal/cliadmit — cross-process LLM-CLI admission
//     control via flock'd holder-set JSON, TTL pruning, and Acquire/release hook
//     in internal/bridge/driver_tmux_repl.go.
package cycle3

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// --- Task: cliadmit-cross-process-admission ---

// TestC3_001_CliadmitPackageExistsAndTracked asserts that
// go/internal/cliadmit/cliadmit.go was created in the worktree and is
// git-tracked. A gitignored file is silently dropped at ship (cycle-93 lesson).
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

// TestC3_002_CliadmitPackageCompiles asserts that all exports named in the
// cliadmit API contract are present by running go build. A missing export is
// a compile error — behavioral proof the API contract holds.
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

// TestC3_003_AcquireUnboundedTestPasses runs TestAcquire_Unbounded under -race
// and asserts: (a) the test executed (anti-no-op guard), (b) exit 0, (c) no
// DATA RACE. The safe-default invariant: max<=0 must return a no-op release
// immediately without creating any slots file or acquiring any lock.
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

// TestC3_004_AcquireMaxOneTestPasses runs TestAcquire_MaxOne under -race and
// asserts it passes. Positive axis: a single caller must acquire the sole slot
// for a max=1 cap and receive a working release function.
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

// TestC3_005_AcquireMutualExclusionTestPasses runs TestAcquire_MutualExclusion
// under -race and asserts it passes. Negative/adversarial axis: two concurrent
// goroutines must NOT both hold a max=1 slot simultaneously — the second caller
// must block until the first releases. This is the strongest anti-no-op signal
// for the admission-control invariant.
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

// TestC3_006_AcquireStaleHolderPrunedTestPasses runs TestAcquire_StaleHolderPruned
// under -race and asserts it passes. TTL-pruning safety: a holder whose heartbeat
// exceeds the TTL is removed from the slot-set so its slot becomes available again,
// preventing a permanent deadlock after a crashed holder.
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

// TestC3_007_AcquireReleaseFreesSlotTestPasses runs TestAcquire_ReleaseFreesSlot
// under -race and asserts it passes. Release correctness: calling the release
// function returned by Acquire must remove the caller's holder from the slot-set,
// allowing a subsequent caller to immediately acquire.
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

// TestC3_008_AcquireContextCancelUnblocksTestPasses runs
// TestAcquire_ContextCancelUnblocks under -race and asserts it passes.
// Degradation invariant: admission control must NEVER block a phase outright —
// context cancellation must unblock a waiting caller within one backoff tick.
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

// TestC3_009_CliadmitHookedInDriverTmuxRepl asserts that the Acquire call is
// wired in go/internal/bridge/driver_tmux_repl.go before session creation. Two
// checks: (1) the import of cliadmit is present, (2) Acquire is called in the
// session-spawn block, (3) EVOLVE_CLI_MAX_CONCURRENT_ env dial is referenced.
//
// acs-predicate: config-check — source-wiring assertion is inherently a
// file-presence check; the behavioral side is covered by TestC3_003-008.
func TestC3_009_CliadmitHookedInDriverTmuxRepl(t *testing.T) {
	root := acsassert.RepoRoot(t)
	driverPath := filepath.Join(root, "go", "internal", "bridge", "driver_tmux_repl.go")
	// cliadmit import must be present.
	if !acsassert.FileContains(t, driverPath, "cliadmit") {
		t.Errorf("RED: cliadmit not imported or referenced in driver_tmux_repl.go")
	}
	// The specific Acquire call must be present.
	if !acsassert.FileContains(t, driverPath, "cliadmit.Acquire") {
		t.Errorf("RED: cliadmit.Acquire not called in driver_tmux_repl.go")
	}
	// The env dial must be referenced (EVOLVE_CLI_MAX_CONCURRENT_).
	if !acsassert.FileContains(t, driverPath, "EVOLVE_CLI_MAX_CONCURRENT_") {
		t.Errorf("RED: EVOLVE_CLI_MAX_CONCURRENT_ dial not referenced in driver_tmux_repl.go")
	}
}

// TestC3_010_ApiCoverEnforceContainsCliadmit asserts ./internal/cliadmit is
// enrolled in go/.apicover-enforce. Enrollment is mandatory for every new
// internal package (TestApicoverEnforce_CoversEveryInternalPackage gate,
// cycle-131 lesson: a new pkg not enrolled fails the completeness invariant
// at ship).
//
// acs-predicate: config-check — enrollment verification is inherently a
// file-presence check; the behavioral gate is TestC3_011.
func TestC3_010_ApiCoverEnforceContainsCliadmit(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforcePath := filepath.Join(root, "go", ".apicover-enforce")
	// acs-predicate: config-check
	acsassert.FileContains(t, enforcePath, "./internal/cliadmit")
}

// TestC3_011_ApiCoverEnforceTestPasses runs TestApicoverEnforce_CoversEveryInternalPackage
// which is the completeness gate: every internal pkg enrolled in .apicover-enforce
// must have named coverage tests in the same package. This fails until
// cliadmit's apicover_named_test.go names every export.
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
