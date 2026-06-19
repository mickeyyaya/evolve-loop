//go:build acs

// Package cycle14 materializes the cycle-3 acceptance criteria for three committed tasks:
//
//   - fix-h1-fanout-bounds — add MaxCycles (50) and MaxWaveWidth (10) constants and
//     guards to campaign.Verify(); add --concurrency flag to `evolve campaign run`.
//
//   - fix-h2-project-root-binding — add --project-root flag to `evolve campaign run`
//     and extend cycleRunArgs to thread it through to child `evolve cycle run` calls.
//
//   - fix-m1-cmd-campaign-coverage — create go/cmd/evolve/cmd_campaign_test.go with
//     table-driven tests covering cycleFromWorkspace, renderCampaignPlan,
//     loadVerifiedCampaignPlan, and localized-retry paths.
//
// AC map (1:1 with scout-report.md ACs, all in top_n):
//
//	fix-h1-fanout-bounds:
//	  AC1  Verify rejects a plan with > MaxCycles (50) cycles              → C3_001
//	  AC1- Verify accepts a plan at exactly MaxCycles (50) cycles          → C3_001neg (pre-existing GREEN)
//	  AC2  Verify rejects a plan with wave width > MaxWaveWidth (10)       → C3_002
//	  AC2- Verify accepts a plan at exactly MaxWaveWidth (10) wide         → C3_002neg (pre-existing GREEN)
//	  AC3  `evolve campaign run` recognizes --concurrency flag             → C3_003
//	  AC3- --concurrency flag is optional (absent → no flag error)         → C3_003neg
//
//	fix-h2-project-root-binding:
//	  AC4  `evolve campaign run` recognizes --project-root flag            → C3_004
//	  AC4- --project-root is optional (absent → no flag error)             → C3_004neg
//
//	fix-m1-cmd-campaign-coverage:
//	  AC5  cmd_campaign_test.go exists, is git-tracked, covers TestCampaign → C3_005
//	  AC5- cmd_campaign_test.go covers cycleFromWorkspace function          → C3_005neg
//
// RED criteria: C3_001, C3_002, C3_003, C3_004, C3_005, C3_005neg.
// Pre-existing GREEN: C3_001neg, C3_002neg, C3_003neg, C3_004neg.
//
// Note on C3_003/C3_004: these use `go run ./cmd/evolve` (not the prebuilt
// binary) so they test current source regardless of binary staleness. The
// binary at go/bin/evolve may be from a prior commit; go run always compiles
// the HEAD source and correctly surfaces missing flags.
//
// Floor binding (R9.3): predicates only for tasks in triage top_n.
package cycle14

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goRunEvolve compiles the evolve binary from current source (go run ./cmd/evolve)
// and runs it with the given args. Returns combined stdout+stderr and exit code.
// This is used for C3_003/C3_004 where we must test the current source rather
// than a potentially-stale prebuilt binary.
func goRunEvolve(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	quotedArgs := make([]string, len(args))
	for i, a := range args {
		quotedArgs[i] = "'" + strings.ReplaceAll(a, "'", "'\\''") + "'"
	}
	bashCmd := "cd '" + goDir + "' && go run ./cmd/evolve " + strings.Join(quotedArgs, " ")
	out, errOut, c, _ := acsassert.SubprocessOutput("bash", "-c", bashCmd)
	return strings.TrimSpace(out + "\n" + errOut), c
}

// chainedPlan builds a plan where each cycle depends on the previous — wave
// width stays 1 but total cycle count equals n.
func chainedPlan(t *testing.T, n int) *campaign.Plan {
	t.Helper()
	if n <= 0 {
		t.Fatalf("chainedPlan: n must be > 0, got %d", n)
	}
	cycles := make([]fleet.Todo, n)
	for i := 0; i < n; i++ {
		cycles[i] = fleet.Todo{ID: fmt.Sprintf("c%d", i+1), Files: []string{fmt.Sprintf("f%d", i+1)}}
		if i > 0 {
			cycles[i].DependsOn = []string{cycles[i-1].ID}
		}
	}
	return &campaign.Plan{Version: 1, Goal: "test-goal", Cycles: cycles}
}

// independentPlan builds a plan where all n cycles are independent (no deps),
// so they all land in wave 0 with wave width = n.
func independentPlan(t *testing.T, n int) *campaign.Plan {
	t.Helper()
	if n <= 0 {
		t.Fatalf("independentPlan: n must be > 0, got %d", n)
	}
	cycles := make([]fleet.Todo, n)
	for i := 0; i < n; i++ {
		cycles[i] = fleet.Todo{ID: fmt.Sprintf("c%d", i+1), Files: []string{fmt.Sprintf("f%d", i+1)}}
	}
	return &campaign.Plan{Version: 1, Goal: "test-goal", Cycles: cycles}
}

// ── fix-h1-fanout-bounds ────────────────────────────────────────────────────

// TestC3_001_VerifyRejectsExceedingMaxCycles verifies that campaign.Verify()
// returns a non-nil error for a plan whose cycle count exceeds MaxCycles (50).
//
// BEHAVIORAL: calls campaign.Verify() directly on a 51-cycle plan.
// A no-op (or unchanged) Verify cannot satisfy this — a real bounds check is
// required. If the implementation deletes lines, this test fails.
//
// RED: current Verify has no MaxCycles check → returns nil for 51 cycles.
// GREEN: Builder adds MaxCycles=50 guard and Verify returns an error.
func TestC3_001_VerifyRejectsExceedingMaxCycles(t *testing.T) {
	plan := chainedPlan(t, 51) // 1 over the MaxCycles=50 limit
	if err := plan.Verify(); err == nil {
		t.Errorf("RED: campaign.Verify() on a 51-cycle plan returned nil.\n" +
			"Builder must add a MaxCycles=50 constant and a bounds check in Verify():\n" +
			"  if len(p.Cycles) > MaxCycles { return fmt.Errorf(\"campaign: %%d cycles exceeds max %%d\", ...) }\n" +
			"File: go/internal/campaign/campaign.go")
	}
}

// TestC3_001neg_VerifyAcceptsAtMaxCyclesLimit verifies that a plan with exactly
// MaxCycles (50) cycles passes Verify — boundary condition / off-by-one guard.
//
// PRE-EXISTING GREEN: Verify has no limit today → 50 cycles pass.
// After Builder: Verify accepts ≤50 cycles, so this continues to pass.
func TestC3_001neg_VerifyAcceptsAtMaxCyclesLimit(t *testing.T) {
	plan := chainedPlan(t, 50) // exactly at the MaxCycles=50 limit
	if err := plan.Verify(); err != nil {
		t.Errorf("FAIL: campaign.Verify() rejected a 50-cycle plan (should be at-limit accepted): %v", err)
	}
}

// TestC3_002_VerifyRejectsExceedingMaxWaveWidth verifies that campaign.Verify()
// returns a non-nil error for a plan whose first wave has width > MaxWaveWidth (10).
//
// BEHAVIORAL: calls campaign.Verify() directly on a plan with 11 independent
// cycles (no deps), so they all land in wave 0 — wave width = 11.
//
// RED: current Verify has no MaxWaveWidth check → returns nil for 11-wide wave.
// GREEN: Builder adds MaxWaveWidth=10 guard after fleet.PlanWaves() call.
func TestC3_002_VerifyRejectsExceedingMaxWaveWidth(t *testing.T) {
	plan := independentPlan(t, 11) // wave width = 11 > MaxWaveWidth=10
	if err := plan.Verify(); err == nil {
		t.Errorf("RED: campaign.Verify() on a plan with 11 independent cycles (wave width 11) returned nil.\n" +
			"Builder must add a MaxWaveWidth=10 constant and a per-wave width check in Verify():\n" +
			"  for _, wave := range waves { if len(wave) > MaxWaveWidth { return fmt.Errorf(...) } }\n" +
			"File: go/internal/campaign/campaign.go")
	}
}

// TestC3_002neg_VerifyAcceptsAtMaxWaveWidthLimit verifies that a plan with
// exactly MaxWaveWidth (10) independent cycles in wave 0 passes Verify.
//
// PRE-EXISTING GREEN: no wave width limit today → 10 independent cycles pass.
// After Builder: guard is >10 so exactly 10 continues to pass.
func TestC3_002neg_VerifyAcceptsAtMaxWaveWidthLimit(t *testing.T) {
	plan := independentPlan(t, 10) // wave width = 10 = MaxWaveWidth (at limit)
	if err := plan.Verify(); err != nil {
		t.Errorf("FAIL: campaign.Verify() rejected a 10-wide plan (should be at-limit accepted): %v", err)
	}
}

// TestC3_003_CampaignRunRecognizesConcurrencyFlag verifies that
// `evolve campaign run --concurrency 1 --plan /dev/null` does NOT produce
// "flag provided but not defined: -concurrency".
//
// BEHAVIORAL: uses `go run ./cmd/evolve` to compile from current source (not
// the prebuilt binary which may be stale). This makes the predicate correct
// regardless of binary staleness — only the source matters.
//
// RED: --concurrency flag absent in runCampaignRun's flag.FlagSet →
//
//	go run produces "flag provided but not defined: -concurrency".
//
// GREEN: Builder adds fs.IntVar(&concurrency, "concurrency", ...) to
// runCampaignRun in go/cmd/evolve/cmd_campaign.go.
func TestC3_003_CampaignRunRecognizesConcurrencyFlag(t *testing.T) {
	combined, _ := goRunEvolve(t, "campaign", "run", "--concurrency", "1", "--plan", "/dev/null")
	if strings.Contains(combined, "flag provided but not defined") &&
		strings.Contains(combined, "concurrency") {
		t.Errorf("RED: `evolve campaign run --concurrency 1 --plan /dev/null` produced\n"+
			"\"flag provided but not defined: -concurrency\".\n"+
			"Builder must add a --concurrency flag to runCampaignRun in go/cmd/evolve/cmd_campaign.go:\n"+
			"  fs.IntVar(&concurrency, \"concurrency\", campaign.MaxWaveWidth, \"max concurrent cycles\")\n"+
			"Output:\n%s", combined)
	}
}

// TestC3_003neg_ConcurrencyFlagIsOptionalNotRequired verifies that
// `evolve campaign run --plan /dev/null` (without --concurrency) does not
// trigger any flag error — --concurrency must be optional with a default.
//
// BEHAVIORAL: compiles from source and confirms no flag validation failure.
// If the flag is required (no default), this would fail with a missing-flag error.
func TestC3_003neg_ConcurrencyFlagIsOptionalNotRequired(t *testing.T) {
	combined, _ := goRunEvolve(t, "campaign", "run", "--plan", "/dev/null")
	if strings.Contains(combined, "flag provided but not defined") {
		t.Errorf("FAIL: `evolve campaign run --plan /dev/null` triggered an unexpected flag error.\n"+
			"The --concurrency flag must be OPTIONAL with a default value.\nOutput:\n%s", combined)
	}
}

// ── fix-h2-project-root-binding ─────────────────────────────────────────────

// TestC3_004_CampaignRunRecognizesProjectRootFlag verifies that
// `evolve campaign run --project-root /tmp --plan /dev/null` does NOT produce
// "flag provided but not defined: -project-root".
//
// BEHAVIORAL: uses `go run ./cmd/evolve` to compile from current source (not
// the prebuilt binary). This correctly tests the source state before any build.
//
// RED: --project-root absent in runCampaignRun's flag.FlagSet →
//
//	go run produces "flag provided but not defined: -project-root".
//
// GREEN: Builder adds fs.StringVar(&projectRoot, "project-root", "", ...) to
// runCampaignRun and extends cycleRunArgs(goalHash, simulate, projectRoot).
func TestC3_004_CampaignRunRecognizesProjectRootFlag(t *testing.T) {
	combined, _ := goRunEvolve(t, "campaign", "run", "--project-root", "/tmp", "--plan", "/dev/null")
	if strings.Contains(combined, "flag provided but not defined") &&
		strings.Contains(combined, "project-root") {
		t.Errorf("RED: `evolve campaign run --project-root /tmp --plan /dev/null` produced\n"+
			"\"flag provided but not defined: -project-root\".\n"+
			"Builder must add a --project-root flag to runCampaignRun in go/cmd/evolve/cmd_campaign.go\n"+
			"and extend cycleRunArgs(goalHash, simulate, projectRoot) in go/cmd/evolve/cmd_fleet.go.\n"+
			"Output:\n%s", combined)
	}
}

// TestC3_004neg_ProjectRootFlagIsOptionalNotRequired verifies that
// `evolve campaign run --plan /dev/null` (no --project-root) fails for a PLAN
// error (empty/invalid), not a flag error — --project-root must be optional.
//
// BEHAVIORAL: compiles from source and confirms no "flag provided but not defined".
func TestC3_004neg_ProjectRootFlagIsOptionalNotRequired(t *testing.T) {
	combined, _ := goRunEvolve(t, "campaign", "run", "--plan", "/dev/null")
	if strings.Contains(combined, "flag provided but not defined") {
		t.Errorf("FAIL: `evolve campaign run --plan /dev/null` triggered an unexpected flag error.\n"+
			"The --project-root flag must be OPTIONAL (default \"\").\nOutput:\n%s", combined)
	}
}

// ── fix-m1-cmd-campaign-coverage ────────────────────────────────────────────

// TestC3_005_CmdCampaignTestFileExistsAndTracked verifies that
// go/cmd/evolve/cmd_campaign_test.go exists on disk, is git-tracked, and
// contains at least one TestCampaign* function.
//
// BEHAVIORAL: disk presence + git tracking (cycle-92 lesson: gitignored files
// are silently dropped at ship) + content check prevents a placeholder file.
// A no-op empty file cannot satisfy the content assertion.
//
// RED: file does not exist yet → all three checks fail.
// GREEN: Builder creates cmd_campaign_test.go with the five coverage tests.
func TestC3_005_CmdCampaignTestFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "cmd", "evolve", "cmd_campaign_test.go")
	abs := filepath.Join(root, rel)

	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing on disk.\n"+
			"Builder must create go/cmd/evolve/cmd_campaign_test.go with table-driven\n"+
			"tests for cycleFromWorkspace, renderCampaignPlan, loadVerifiedCampaignPlan,\n"+
			"and the localized-retry happy-path and terminal-failure paths.", rel)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	if code != 0 {
		t.Errorf("RED: %s exists on disk but is not git-tracked (may be gitignored — dropped at ship).\n"+
			"Builder must `git add go/cmd/evolve/cmd_campaign_test.go`.", rel)
	}
	if !acsassert.FileContains(t, abs, "TestCampaign") {
		t.Errorf("RED: %s does not contain any TestCampaign* function.\n"+
			"Builder must add table-driven TestCampaign* tests to the file.", rel)
	}
}

// TestC3_005neg_TestFileCoversCycleFromWorkspace is the adversarial negative:
// the test file must explicitly exercise cycleFromWorkspace — a file that only
// has TestCampaignRun_* tests without testing the workspace-to-cycle-number
// extraction cannot satisfy M1's stated coverage requirement.
//
// RED: file does not exist → FileContains fails on read error.
// GREEN: Builder's cmd_campaign_test.go contains a cycleFromWorkspace test case.
func TestC3_005neg_TestFileCoversCycleFromWorkspace(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "cmd", "evolve", "cmd_campaign_test.go")
	abs := filepath.Join(root, rel)

	if !acsassert.FileContains(t, abs, "cycleFromWorkspace") {
		t.Errorf("RED: %s does not cover cycleFromWorkspace.\n"+
			"Builder must include test cases exercising cycleFromWorkspace with valid\n"+
			"(\"cycle-7\" → 7), non-cycle (\"other\" → 0), and empty-string inputs.\nFile: %s",
			rel, abs)
	}
}
