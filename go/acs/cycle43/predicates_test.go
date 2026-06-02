//go:build legacy

// Package cycle43 ports the cycle-43 ACS predicates (10 bash files)
// from acs/cycle-43/*.sh to Go test counterparts using pkg/acsassert.
//
// Coexistence note (parent plan §4 Phase 4): the bash predicates stay
// in place at acs/cycle-43/*.sh. These Go tests run against the same
// repo state and assert the same invariants. acsrunner picks them up
// via `go test -json ./acs/cycle43/...`.
package cycle43

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// Local aliases keep the cycle-43 tests reading the same as before,
// while the implementation lives in pkg/acsassert (shared by all cycle
// packages — see Phase 3 task #15).
func repoRoot(t *testing.T) string { return acsassert.RepoRoot(t) }
func fileContainsAny(path string, variants ...string) bool {
	return acsassert.FileContainsAny(path, variants...)
}
func countOccurrencesAny(path string, variants ...string) int {
	return acsassert.CountOccurrencesAny(path, variants...)
}

// TestC43_001_PNew17InvestigationComplete ports cycle-43/001-p-new-17.
// Verifies P-NEW-17 status is INVESTIGATION-COMPLETE in roadmap.
func TestC43_001_PNew17InvestigationComplete(t *testing.T) {
	root := repoRoot(t)
	roadmap := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !fixtures.FilePresent(roadmap) {
		t.Skip("roadmap missing — skip cycle-43-001")
	}
	if !acsassert.FileContains(t, roadmap, "INVESTIGATION-COMPLETE") {
		return
	}
}

// TestC43_001b_RetrospectiveYAMLContract ports the
// retrospective-yaml-contract variant of cycle-43-001.
func TestC43_001b_RetrospectiveYAMLContract(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "agents", "evolve-retrospective.md")
	if !fixtures.FilePresent(file) {
		t.Skip("evolve-retrospective.md missing — skip cycle-43-001b")
	}
	if !acsassert.FileContains(t, file, "MUST-FIRST") {
		return
	}
	if !acsassert.FileContains(t, file, "INTEGRITY_FAIL") {
		return
	}
	if !fileContainsAny(file, "dangling IDs", "exit 2") {
		t.Errorf("%s missing 'dangling IDs' or 'exit 2' marker (Final checks contract)", file)
	}
}

// TestC43_002_Cycle42CostSnapshot ports cycle-43/002 cost-snapshot.
func TestC43_002_Cycle42CostSnapshot(t *testing.T) {
	root := repoRoot(t)
	roadmap := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !fixtures.FilePresent(roadmap) {
		t.Skip("roadmap missing — skip cycle-43-002")
	}
	if !acsassert.FileContains(t, roadmap, "Cycle-42 cost snapshot") {
		return
	}
	// $5.80 — escape the literal $ for regex.
	if !acsassert.FileMatchesRegex(t, roadmap, `\$5\.80`) {
		return
	}
}

// TestC43_002b_OrchestratorMergeRCCheck ports the merge-rc-check variant.
func TestC43_002b_OrchestratorMergeRCCheck(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "agents", "evolve-orchestrator.md")
	if !fixtures.FilePresent(file) {
		t.Skip("evolve-orchestrator.md missing — skip cycle-43-002b")
	}
	if !acsassert.FileContains(t, file, "MERGE_RC=$?") {
		return
	}
	// Either "MERGE_RC -eq 2 ... exit 2" OR "exit 2 ... INTEGRITY_FAIL"
	if !fileContainsAny(file, "exit 2", "INTEGRITY_FAIL") {
		t.Errorf("%s missing 'exit 2' / 'INTEGRITY_FAIL' marker", file)
	}
}

// TestC43_003_PNew18And19Exist ports cycle-43/003-p-new-18-and-19.
func TestC43_003_PNew18And19Exist(t *testing.T) {
	root := repoRoot(t)
	roadmap := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !fixtures.FilePresent(roadmap) {
		t.Skip("roadmap missing — skip cycle-43-003")
	}
	if !acsassert.FileContains(t, roadmap, "P-NEW-18") {
		return
	}
	if !acsassert.FileContains(t, roadmap, "P-NEW-19") {
		return
	}
}

// TestC43_003b_PhaseGateRetroComplete ports the phase-gate-retro variant.
func TestC43_003b_PhaseGateRetroComplete(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "legacy", "scripts", "lifecycle", "phase-gate.sh")
	if !fixtures.FilePresent(file) {
		t.Skip("phase-gate.sh missing — skip cycle-43-003b")
	}
	if !acsassert.FileMatchesRegex(t, file, `(?m)^gate_retrospective_to_complete\(\)`) {
		return
	}
	if !acsassert.FileContains(t, file, "retrospective-to-complete) gate_retrospective_to_complete") {
		return
	}
	if !fileContainsAny(file, "lessonIds", "INTEGRITY_FAIL", "lesson.*yaml", "yaml.*lesson") {
		t.Errorf("%s missing YAML integrity check markers", file)
	}
}

// TestC43_004_AuditorStopCriterion ports cycle-43/004-auditor-stop.
func TestC43_004_AuditorStopCriterion(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "agents", "evolve-auditor.md")
	if !fixtures.FilePresent(file) {
		t.Skip("evolve-auditor.md missing — skip cycle-43-004")
	}
	if !acsassert.FileContains(t, file, "## STOP CRITERION") {
		return
	}
	gateCount := countOccurrencesAny(file,
		"predicates-run", "verdict-decided", "report-written", "defects-listed")
	if gateCount < 3 {
		t.Errorf("%s has %d named completion gates; want >=3", file, gateCount)
	}
	if !fileContainsAny(file, "Banned Post-Report", "banned post-report", "post-report") {
		t.Errorf("%s missing banned-post-report patterns marker", file)
	}
}

// TestC43_004b_CachePrefixV2DefaultOn ports the cache-prefix variant.
func TestC43_004b_CachePrefixV2DefaultOn(t *testing.T) {
	root := repoRoot(t)
	subagent := filepath.Join(root, "legacy", "scripts", "dispatch", "subagent-run.sh")
	claudeSh := filepath.Join(root, "legacy", "scripts", "cli_adapters", "claude.sh")
	flagsDoc := filepath.Join(root, "docs", "architecture", "control-flags.md")
	if !fixtures.FilePresent(subagent) || !acsassert.FileExists(t, claudeSh) {
		t.Skip("subagent-run.sh or claude.sh missing — skip cycle-43-004b")
	}
	if !acsassert.FileContains(t, subagent, "EVOLVE_CACHE_PREFIX_V2:-1") {
		return
	}
	if !acsassert.FileContains(t, claudeSh, "EVOLVE_CACHE_PREFIX_V2:-1") {
		return
	}
	if acsassert.FileExists(t, flagsDoc) {
		// Both ACTIVE marker AND env-var name must coexist.
		if !acsassert.FileContains(t, flagsDoc, "EVOLVE_CACHE_PREFIX_V2") {
			return
		}
		if !acsassert.FileContains(t, flagsDoc, "ACTIVE (default `1`)") {
			return
		}
	}
}

// TestC43_005_BuilderStopCriterion ports cycle-43/005-builder-stop.
func TestC43_005_BuilderStopCriterion(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "agents", "evolve-builder.md")
	if !fixtures.FilePresent(file) {
		t.Skip("evolve-builder.md missing — skip cycle-43-005")
	}
	if !acsassert.FileContains(t, file, "## STOP CRITERION") {
		return
	}
	gateCount := countOccurrencesAny(file,
		"worktree-verified", "implementation-complete", "self-verify-passed", "report-written")
	if gateCount < 3 {
		t.Errorf("%s has %d named completion gates; want >=3", file, gateCount)
	}
	if !fileContainsAny(file, "Banned Post-Report", "banned post-report", "post-report") {
		t.Errorf("%s missing banned-post-report patterns marker", file)
	}
}

// TestC43_006_RoadmapPNew21To23 ports cycle-43/006-roadmap-p-new-21-to-23.
func TestC43_006_RoadmapPNew21To23(t *testing.T) {
	root := repoRoot(t)
	file := filepath.Join(root, "docs", "architecture", "token-reduction-roadmap.md")
	if !fixtures.FilePresent(file) {
		t.Skip("roadmap missing — skip cycle-43-006")
	}
	for _, id := range []string{"P-NEW-21", "P-NEW-22", "P-NEW-23"} {
		if !acsassert.FileContains(t, file, id) {
			return
		}
	}
	// P-NEW-20 must be DONE. Accept either "P-NEW-20 ... DONE" on one line
	// or the table-cell shape "P-NEW-20 Builder stop-criterion ... DONE".
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read roadmap: %v", err)
	}
	if !regexp.MustCompile(`P-NEW-20[\s\S]{0,200}DONE`).Match(raw) {
		t.Errorf("%s: P-NEW-20 not marked DONE within 200 chars", file)
	}
}
