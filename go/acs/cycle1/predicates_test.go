//go:build acs

// Package cycle1 materializes the cycle-1 acceptance criteria for two tasks:
//
//   - codex-pretrust-concurrent-regression: regression test for concurrent
//     pretrust of distinct worktree paths against a shared
//     EVOLVE_CODEX_CONFIG_PATH file (Slice 2, concurrency-arch-slices campaign).
//
//   - cycle-audit-cycle-scoped-ci-gap: audit gate verification ensuring the
//     gofmt CI-parity gate and SKILL.md drift gate are wired in NewDefault
//     (prevents recurrence of cycles 339-341 CI-red regressions).
package cycle1

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// --- Task: codex-pretrust-concurrent-regression ---

// TestC1_001_ConcurrentTestFileExistsAndTracked asserts that
// go/internal/bridge/codex_pretrust_concurrent_test.go was created in the
// worktree and is git-tracked. Disk presence alone is insufficient — a
// gitignored file is silently dropped at ship (cycle-93 lesson).
func TestC1_001_ConcurrentTestFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "internal", "bridge", "codex_pretrust_concurrent_test.go")
	path := filepath.Join(root, rel)
	if !acsassert.FileExists(t, path) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	// git-tracking check (cycle-93 pattern): untracked files are dropped at ship.
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("RED: %s not git-tracked — may be gitignored and dropped at ship", rel)
	}
}

// TestC1_002_ConcurrentTwoGoroutinesTestPassesWithRace runs
// TestPretrustCodexProjects_ConcurrentTwoGoroutines under -race -count=5 and
// asserts: (a) the test actually executed (anti-no-op: `go test -run` on a
// missing test exits 0 silently), (b) exit 0, (c) no DATA RACE. This is the
// behavioral predicate covering AC2+AC5+AC6: it exercises the actual
// pretrust/flock machinery. A lost-update race that drops one goroutine's TOML
// entry would surface as a test assertion failure or DATA RACE report.
func TestC1_002_ConcurrentTwoGoroutinesTestPassesWithRace(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	// -v is required so the RUN line appears in output — needed for the
	// anti-no-op check below (without -v, a matched-but-passing test leaves
	// no name in stdout and we cannot distinguish "ran+passed" from "not found").
	stdout, stderr, code, _ := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir,
		"-race", "-v", "-count=5",
		"-tags", "integration",
		"-run", "TestPretrustCodexProjects_ConcurrentTwoGoroutines",
		"./internal/bridge/...",
	)
	combined := stdout + "\n" + stderr
	// Anti-no-op: if the test name is absent, the function was never called.
	// "no tests to run" exits 0 — an empty pass that cannot prove the feature.
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

// TestC1_003_ConcurrentTest_NameAndSeamPresent verifies the concurrent test file
// contains the required function name and test seam.
// acs-predicate: config-check — verifying test structure is inherently a
// config-presence check; the behavioral coverage is in TestC1_002.
func TestC1_003_ConcurrentTest_NameAndSeamPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "bridge", "codex_pretrust_concurrent_test.go")
	// Function name matches AC2 (ConcurrentTwoGoroutines or similar _Concurrent* variant).
	acsassert.FileMatchesRegex(t, path, `TestPretrustCodexProjects_Concurrent`)
	// EVOLVE_CODEX_CONFIG_PATH seam prevents real ~/.codex from being touched (AC3).
	acsassert.FileContains(t, path, `EVOLVE_CODEX_CONFIG_PATH`)
}

// TestC1_004_ConcurrentTest_NoProductionFilesModified is a negative predicate
// asserting the test-only slice did not modify any non-test .go file in
// go/internal/bridge/. A test-only slice that touches production code violates
// the "no production change" constraint (AC7).
//
// Negative axis: a no-op predicate that only checks test presence cannot
// detect a rogue production-code change (anti-gaming, SKILL §2).
func TestC1_004_ConcurrentTest_NoProductionFilesModified(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// List all .go files under go/internal/bridge/ that are in the index but
	// NOT suffixed _test.go. If any of these appear in the most-recent commit's
	// diff, the slice violated the test-only constraint.
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
			continue // test files are expected
		}
		t.Errorf("RED: production file modified in test-only slice: %s", line)
	}
}

// --- Task: cycle-audit-cycle-scoped-ci-gap ---

// TestC1_005_GofmtGateWiredInAuditNewDefault verifies the gofmt CI-parity
// gate is active in the deployed NewDefault configuration. Behavioral: runs the
// existing TestNewDefault_WiresGofmtCheck test as a subprocess against the
// worktree's audit package. A dirty go file with a green EGPS suite MUST cause
// Verdict=FAIL — if NewDefault no longer wires the gate, this test exits
// non-zero, and this predicate reports RED.
//
// Prevents recurrence of cycles 339-341: generated go/acs/cycle<N>/*.go files
// that were not gofmt-clean shipped CI-red because the cycle-scoped audit
// never ran gofmt before ship.
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

// TestC1_006_SkillsDriftGateWiredInAuditNewDefault verifies the SKILL.md
// phase-facts drift gate is active in the deployed NewDefault configuration.
// Behavioral: runs the existing TestNewDefault_WiresSkillsDriftCheck test as a
// subprocess. A cycle that edits .evolve/profiles/*.json without regenerating
// SKILL.md must FAIL audit — this predicate confirms the gate is armed.
//
// Prevents recurrence of cycle 339's SKILL.md drift that shipped CI-red on
// TestSkills_NoDrift.
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
