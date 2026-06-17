//go:build acs

// Package cycle354 materializes the cycle-354 acceptance criteria for the
// two committed top_n tasks:
//
//   - fix-cluster-table-dead-flag-status — update 10 hand-maintained cluster
//     table rows in control-flags.md from ACTIVE/DEPRECATED → DEAD (registry=SSOT).
//   - cycle-audit-cycle-scoped-ci-gap — verify that the gofmt CI-parity gate and
//     SKILL.md-drift gate are wired in the audit phase (both fixes already shipped
//     in commits 23582c91 / 7feec764; predicates are pre-existing GREEN regression locks).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	fix-cluster-table-dead-flag-status:
//	  AC1 (neg)  EVOLVE_RESOLVE_ROOTS_LOADED no longer ACTIVE in cluster table → C354_001
//	  AC2 (neg)  EVOLVE_FAILURE_CLASSIFICATIONS_LOADED no longer ACTIVE         → C354_001
//	  AC3 (neg)  GEMINI_CLAUDE_PATH / GEMINI_REQUIRE_FULL no longer ACTIVE      → C354_002
//	  AC4 (neg)  EVOLVE_STRICT_FAILURES no longer DEPRECATED                    → C354_003
//	  AC1 (pos)  At least 5 flags show DEAD in hand-maintained section           → C354_004
//	  AC2        evolve flags check exits 0 (pre-existing GREEN)                 → C354_005
//
//	cycle-audit-cycle-scoped-ci-gap (both pre-existing GREEN):
//	  CA1  gofmt CI-parity gate wired in audit.NewDefault                       → C354_006
//	  CA2  SKILL.md-drift gate wired in audit.NewDefault                        → C354_007
//
// Floor binding (R9.3): only committed top_n items get predicates.
// fix-dynamic-routing-registry-default is DEFERRED; no predicate for it.
package cycle354

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goDir returns the go module directory for use as -C in subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// controlFlagsPath returns the absolute path to control-flags.md.
func controlFlagsPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "docs", "architecture", "control-flags.md")
}

// TestC354_001_CoreInfraFlagsNotActive verifies that the two Core Infrastructure
// dead flags (EVOLVE_RESOLVE_ROOTS_LOADED, EVOLVE_FAILURE_CLASSIFICATIONS_LOADED)
// no longer appear as ACTIVE in the hand-maintained cluster tables of
// control-flags.md (registry marks them dead; the hand-maintained table must agree).
//
// NEGATIVE (anti-gaming): the hand-maintained section uses uppercase "| ACTIVE |"
// while the Generated Flag Index uses lowercase "| dead |", so this check targets
// only the hand-maintained rows that Builder must update.
//
// RED: before Builder's fix, the file contains:
//
//	`EVOLVE_RESOLVE_ROOTS_LOADED` | ACTIVE
//	`EVOLVE_FAILURE_CLASSIFICATIONS_LOADED` | ACTIVE
//
// acs-predicate: config-check
func TestC354_001_CoreInfraFlagsNotActive(t *testing.T) {
	path := controlFlagsPath(t)

	// EVOLVE_RESOLVE_ROOTS_LOADED must not appear as ACTIVE.
	if !acsassert.FileNotContains(t, path, "`EVOLVE_RESOLVE_ROOTS_LOADED` | ACTIVE") {
		t.Errorf("RED: control-flags.md still shows EVOLVE_RESOLVE_ROOTS_LOADED as ACTIVE "+
			"in the hand-maintained cluster table.\n"+
			"Builder must update the Core Infrastructure cluster row to DEAD.\n"+
			"File: %s", path)
	}

	// EVOLVE_FAILURE_CLASSIFICATIONS_LOADED must not appear as ACTIVE.
	if !acsassert.FileNotContains(t, path, "`EVOLVE_FAILURE_CLASSIFICATIONS_LOADED` | ACTIVE") {
		t.Errorf("RED: control-flags.md still shows EVOLVE_FAILURE_CLASSIFICATIONS_LOADED as ACTIVE "+
			"in the hand-maintained cluster table.\n"+
			"Builder must update the Core Infrastructure cluster row to DEAD.\n"+
			"File: %s", path)
	}
}

// TestC354_002_PlatformHybridFlagsNotActive verifies that the five Platform/CLI Hybrid
// dead flags (GEMINI_CLAUDE_PATH, GEMINI_REQUIRE_FULL, CODEX_CLAUDE_PATH,
// ALLOW_INTERACTIVE_FALLBACK, FORCE_BARE) no longer appear as ACTIVE in the
// hand-maintained cluster tables.
//
// NEGATIVE: uppercase "| ACTIVE |" targets the hand-maintained section only.
// RED: before Builder's fix, all five rows show ACTIVE.
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

// TestC354_003_StrictFailuresNotDeprecated verifies that EVOLVE_STRICT_FAILURES
// no longer appears as DEPRECATED in the hand-maintained Workflow Defaults cluster
// (it has no reader → registry marks it StatusDead, doc must match).
//
// NEGATIVE: the hand-maintained section uses uppercase "| DEPRECATED |".
// RED: before Builder's fix, the row reads "EVOLVE_STRICT_FAILURES | DEPRECATED".
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

// TestC354_004_DeadFlagsShowDeadInClusterTable is the POSITIVE companion to the
// C354_001-003 negative checks. It verifies that at least 5 of the 10 dead flags
// actually appear with "| DEAD |" status in the hand-maintained section after
// Builder's edit (as opposed to just having their ACTIVE row removed with no
// replacement — the table would be accurate either way, but the positive check
// confirms Builder updated the row status rather than deleting the row entirely).
//
// POSITIVE: "| DEAD |" uppercase appears ONLY in the hand-maintained section
// (Generated Flag Index uses lowercase "| dead |").
//
// Uses CountOccurrencesAny (no TB / no internal Errorf) for aggregated counting
// so spurious per-flag errors don't fire; the single Fatalf below is authoritative.
//
// RED: before Builder's fix, none of the 10 flags have "| DEAD |" in the
// hand-maintained section (they all show ACTIVE or DEPRECATED). count == 0 < 5.
// acs-predicate: config-check
func TestC354_004_DeadFlagsShowDeadInClusterTable(t *testing.T) {
	path := controlFlagsPath(t)
	count := acsassert.CountOccurrencesAny(path,
		"`EVOLVE_RESOLVE_ROOTS_LOADED` | DEAD",
		"`EVOLVE_FAILURE_CLASSIFICATIONS_LOADED` | DEAD",
		"`EVOLVE_GEMINI_CLAUDE_PATH` | DEAD",
		"`EVOLVE_GEMINI_REQUIRE_FULL` | DEAD",
		"`EVOLVE_CODEX_CLAUDE_PATH` | DEAD",
		"`EVOLVE_ALLOW_INTERACTIVE_FALLBACK` | DEAD",
		"`EVOLVE_FORCE_BARE` | DEAD",
		"`EVOLVE_STRICT_FAILURES` | DEAD",
		"`EVOLVE_DRY_RUN_PROVISION_WORKTREE` | DEAD",
		"`EVOLVE_PROFILE_WORKTREE_AWARE` | DEAD",
	)
	if count < 5 {
		t.Errorf("RED: expected at least 5 of the 10 dead flags to show DEAD (uppercase) "+
			"in the hand-maintained cluster tables; got %d.\n"+
			"Builder must update all 10 cluster table rows from ACTIVE/DEPRECATED → DEAD.\n"+
			"File: %s", count, path)
	}
}

// TestC354_005_FlagsCheckExitsZero verifies that `evolve flags check` exits 0,
// confirming that the Generated Flag Index in control-flags.md is in sync with
// the flagregistry after any changes Builder makes.
//
// NOTE: this predicate is pre-existing GREEN (the generated index already
// correctly reflects the registry's dead/active status). It will temporarily
// become RED mid-fix if Builder edits registry_table.go without regenerating the
// index, then GREEN again after `evolve flags generate`.
//
// BEHAVIORAL: runs the real evolve binary; source edits alone cannot satisfy it.
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

// TestC354_006_AuditGofmtGateIsWired verifies that the gofmt CI-parity gate is
// wired into audit.NewDefault (the fix for cycles 339-341 shipping CI-red when
// generated go/acs/cycle<N>/predicates_test.go had gofmt diffs).
//
// BEHAVIORAL: runs the audit package test `TestNewDefault_WiresGofmtCheck` which
// creates a real gofmt-dirty .go file in a temp worktree and asserts that
// audit.NewDefault's Run returns VerdictFAIL. Source-only changes cannot satisfy
// this — the seam must be wired in production config.
//
// NOTE: pre-existing GREEN (fix committed in 23582c91 on 2026-06-15).
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

// TestC354_007_AuditSkillsDriftGateIsWired verifies that the SKILL.md-drift gate
// is wired into audit.NewDefault (the fix for cycles 339-341 shipping CI-red when
// .evolve/profiles/*.json edits caused SKILL.md drift).
//
// BEHAVIORAL: runs `TestNewDefault_WiresSkillsDriftCheck` which creates a drifted
// SKILL.md fixture and asserts VerdictFAIL. A grep-over-source alone cannot make
// it pass — the seam must fire in the actual Run path.
//
// NOTE: pre-existing GREEN (fix committed in 7feec764 on 2026-06-15).
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
