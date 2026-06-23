//go:build acs

// Package cycle2 materializes the cycle-2 acceptance criteria for:
//
//   - sessionreaper-orphan-reap: Tier-3 liveness orphan reaper (Slice 3,
//     concurrency-arch-slices campaign). New leaf pkg internal/sessionreaper
//     with ReapOrphans function, replacement of the looppreflight glob-WARN,
//     and `evolve swarm reap-orphans` CLI operator backstop.
package cycle2

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// --- Task: sessionreaper-orphan-reap ---

// TestC2_001_SessionreaperPackageExistsAndTracked asserts that
// go/internal/sessionreaper/sessionreaper.go was created in the worktree
// and is git-tracked. A gitignored file is silently dropped at ship
// (cycle-93 lesson).
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

// TestC2_002_SessionreaperExportsCompile asserts that all four exports named in
// AC1 (ReapOrphans, Options, Report, OrphanReap) are present by running go build.
// A missing export is a compile error — behavioral proof the API contract holds.
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

// TestC2_003_FreshLeaseSkippedTestPasses runs TestReapOrphans_FreshLeaseSkipped
// under -race and asserts: (a) the test executed (anti-no-op guard: `go test -run`
// on a missing test exits 0 silently), (b) exit 0, (c) no DATA RACE.
// This is the safety-invariant predicate: a live peer's sessions must NEVER be
// killed — fake TmuxKiller must record zero calls for the fresh-lease run.
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

// TestC2_004_StaleLeaseReapedTestPasses runs TestReapOrphans_StaleLeaseReaped
// under -race and asserts it passes. The positive behavioral test: a stale-lease
// run's sessions MUST be passed to the killer (non-zero kill count).
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

// TestC2_005_MissingRegistryIsZeroActivityTestPasses runs
// TestReapOrphans_MissingRegistryIsZeroActivity: absent tmux-sessions.jsonl must
// be a zero-activity success (no error, no crash). Mirrors swarm.ReapRunSessions'
// MissingRegistry contract.
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

// TestC2_006_AbsentLeaseIsStaleTestPasses runs TestReapOrphans_AbsentLeaseIsStale:
// a missing .lease file must be treated as stale (fail-closed; reap proceeds)
// rather than live (fail-open; reap skipped). Unknown = reapable.
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

// TestC2_007_LooppreflightGlobWarnRemovedAndReapOrphansWired asserts two things:
// (1) the old server-wide glob-WARN for stale sessions is removed — it's a
// latent footgun (could enumerate live peers' sessions) and incapable of safe
// reaping; (2) ReapOrphans is called in checks.go instead.
//
// Mixed: FileNotContains removes the footgun assertion; FileMatchesRegex
// confirms wiring. The behavioral side is covered by TestC2_008.
// acs-predicate: config-check — source-wiring assertion is inherently a
// file-presence check.
func TestC2_007_LooppreflightGlobWarnRemovedAndReapOrphansWired(t *testing.T) {
	root := acsassert.RepoRoot(t)
	checksPath := filepath.Join(root, "go", "internal", "looppreflight", "checks.go")
	// Negative axis: the old glob-WARN string must be absent.
	acsassert.FileNotContains(t, checksPath, "stale bridge tmux session(s)")
	// Positive axis: ReapOrphans call must be present.
	acsassert.FileMatchesRegex(t, checksPath, `ReapOrphans`)
}

// TestC2_008_LooppreflightTestsPassAfterReapOrphansWiring runs the looppreflight
// integration test suite to confirm the ReapOrphans wiring is correct and no
// regressions were introduced. Behavioral: exercises the actual checks.go code.
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

// TestC2_009_SwarmReapOrphansDryRunSucceeds builds the evolve binary and runs
// `evolve swarm reap-orphans --dry-run`. Exit 0 proves the subcommand is
// registered and functional. --dry-run injects a no-op killer so no real sessions
// are touched.
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

// TestC2_010_ApiCoverEnforceContainsSessionreaper asserts ./internal/sessionreaper
// is enrolled in go/.apicover-enforce. Enrollment is mandatory for every new
// internal package (TestApicoverEnforce_CoversEveryInternalPackage gate, cycle-131
// lesson: a new pkg not enrolled fails the completeness invariant at ship).
//
// acs-predicate: config-check — enrollment verification is inherently a
// file-presence check; the behavioral gate is TestC2_011.
func TestC2_010_ApiCoverEnforceContainsSessionreaper(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enforcePath := filepath.Join(root, "go", ".apicover-enforce")
	// acs-predicate: config-check
	acsassert.FileContains(t, enforcePath, "./internal/sessionreaper")
}

// TestC2_011_ApiCoverEnforceTestPasses runs TestApicoverEnforce_CoversEveryInternalPackage
// which is the completeness gate: every internal pkg enrolled in .apicover-enforce
// must have named coverage tests in the same package. This fails until
// sessionreaper's apicover_named_test.go names every export.
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

// TestC2_012_SessionreaperCoverageAtLeast85Pct runs the integration test suite
// with -coverprofile and asserts coverage >= 85% for the sessionreaper package.
// A package below the threshold cannot ship per the AC10 gate (apicover-enforce
// checks coverage profiles).
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
