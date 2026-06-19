//go:build acs

// Package cycle17 materializes the cycle-17 acceptance criteria for three committed tasks:
//
//   - campaign-verify-comma-rejection — add guard to campaign.Verify() rejecting cycle IDs
//     that contain commas. EVOLVE_FLEET_SCOPE splits on ',' so a cycle ID "a,b" silently
//     fans into two scope tokens, causing incorrect fan-out.
//
//   - fix-preliminary-study-metadata — create agents/evolve-preliminary-study.md persona
//     file to pair with .evolve/profiles/preliminary-study.json, and add when_to_use /
//     description fields to .evolve/phases/preliminary-study/phase.json.
//
//   - cmd-campaign-dispatch-coverage — add TestRunCampaign_NoArgs and
//     TestRenderCampaignPlan_Valid to cmd_campaign_test.go to cover dispatch guards
//     (0% coverage) and the renderCampaignPlan success path (currently 40%).
//
// AC map (1:1 with scout-report.md ACs, all in triage top_n):
//
//	campaign-verify-comma-rejection:
//	  AC1  Verify rejects "b,c" cycle ID (comma delimiter collision)      → C17_001
//	  AC1- Valid plan without commas still passes Verify                  → C17_001neg (pre-existing GREEN)
//
//	fix-preliminary-study-metadata:
//	  AC2  agents/evolve-preliminary-study.md exists and is git-tracked   → C17_002
//	  AC3  phasecoherence.TestRepoPersonaProfilePairing passes            → C17_003
//	  AC4  phasespec.TestPhaseCatalog_OptionalPhasesHaveSelectMetadata passes → C17_004
//
//	cmd-campaign-dispatch-coverage:
//	  AC5  TestRunCampaign_NoArgs exists in cmd_campaign_test.go and passes  → C17_005
//	  AC5- TestRunCampaign_UnknownSub exists and passes (semantic diversity) → C17_005neg
//	  AC6  TestRenderCampaignPlan_Valid exists in cmd_campaign_test.go and passes → C17_006
//
// RED criteria: C17_001, C17_002, C17_003, C17_004, C17_005, C17_005neg, C17_006.
// Pre-existing GREEN: C17_001neg.
//
// Floor binding (R9.3): predicates only for tasks in triage top_n.
package cycle17

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/campaign"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// goTestRun runs `go test` args from the go/ subdirectory of the repo root.
// Returns combined stdout+stderr and exit code.
func goTestRun(t *testing.T, args ...string) (combined string, code int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	goDir := filepath.Join(root, "go")
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", "'\\''") + "'"
	}
	bashCmd := "cd '" + goDir + "' && go test " + strings.Join(quoted, " ")
	out, errOut, c, _ := acsassert.SubprocessOutput("bash", "-c", bashCmd)
	return strings.TrimSpace(out + "\n" + errOut), c
}

// ── campaign-verify-comma-rejection ─────────────────────────────────────────

// TestC17_001_VerifyRejectsCommaInCycleID verifies that campaign.Verify()
// returns a non-nil error when a cycle ID contains a comma.
//
// BEHAVIORAL: calls campaign.Verify() directly on a plan where cycle "b,c"
// contains a comma. EVOLVE_FLEET_SCOPE splits scope tokens on ',' — if a
// cycle ID contains a comma, it silently fans into two scope tokens, which
// corrupts downstream fleet dispatch.
//
// RED: current Verify() only calls strings.TrimSpace — no comma check exists.
//
//	plan.Verify() returns nil for "b,c" today.
//
// GREEN: Builder adds strings.ContainsAny(c.ID, ",") guard in Verify().
func TestC17_001_VerifyRejectsCommaInCycleID(t *testing.T) {
	plan := &campaign.Plan{
		Version: 1,
		Goal:    "test-comma-rejection",
		Cycles: []fleet.Todo{
			{ID: "a", Files: []string{"fa"}},
			{ID: "b,c", Files: []string{"fb"}}, // comma — fleet scope delimiter
		},
	}
	if err := plan.Verify(); err == nil {
		t.Errorf("RED: campaign.Verify() accepted cycle ID %q which contains a comma.\n"+
			"EVOLVE_FLEET_SCOPE splits on ',' so \"b,c\" fans into two scope tokens.\n"+
			"Builder must add in campaign.Verify(), inside the seen-loop after duplicate check:\n"+
			"  if strings.ContainsAny(c.ID, \",\") {\n"+
			"      return fmt.Errorf(\"campaign: cycle id %%q contains invalid char\", c.ID)\n"+
			"  }\n"+
			"File: go/internal/campaign/campaign.go", "b,c")
	}
}

// TestC17_001neg_ValidPlanPassesVerify verifies that a plan with clean (comma-free)
// cycle IDs still passes Verify after the comma guard is added.
//
// PRE-EXISTING GREEN: Verify() returns nil for valid IDs today.
// After Builder: the new guard only fires on commas; valid IDs continue to pass.
func TestC17_001neg_ValidPlanPassesVerify(t *testing.T) {
	plan := &campaign.Plan{
		Version: 1,
		Goal:    "test-valid-ids",
		Cycles: []fleet.Todo{
			{ID: "phase-1", Files: []string{"fa"}},
			{ID: "phase-2", Files: []string{"fb"}, DependsOn: []string{"phase-1"}},
		},
	}
	if err := plan.Verify(); err != nil {
		t.Errorf("FAIL: campaign.Verify() rejected a valid plan (no commas in IDs): %v", err)
	}
}

// ── fix-preliminary-study-metadata ──────────────────────────────────────────

// TestC17_002_PersonaFileExistsAndTracked verifies that
// agents/evolve-preliminary-study.md exists on disk and is git-tracked.
//
// BEHAVIORAL: disk presence check + git ls-files tracking check (cycle-93 pattern:
// gitignored files are silently dropped at ship).
//
// RED: agents/evolve-preliminary-study.md does not exist yet.
//
//	.evolve/profiles/preliminary-study.json exists but has no paired persona.
//
// GREEN: Builder creates agents/evolve-preliminary-study.md and `git add`s it.
func TestC17_002_PersonaFileExistsAndTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("agents", "evolve-preliminary-study.md")
	abs := filepath.Join(root, rel)

	if !acsassert.FileExists(t, abs) {
		t.Fatalf("RED: %s missing on disk.\n"+
			".evolve/profiles/preliminary-study.json exists but has no paired persona.\n"+
			"phasecoherence.TestRepoPersonaProfilePairing will fail at CI.\n"+
			"Builder must create agents/evolve-preliminary-study.md with frontmatter\n"+
			"(name, description, when_to_use) and a brief agent prompt body.", rel)
	}
	_, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel)
	if code != 0 {
		t.Errorf("RED: %s exists on disk but is not git-tracked — may be gitignored and dropped at ship.\n"+
			"Builder must `git add agents/evolve-preliminary-study.md`.", rel)
	}
}

// TestC17_003_PhaseCoherencePasses verifies that
// phasecoherence.TestRepoPersonaProfilePairing passes (exit 0).
//
// BEHAVIORAL: subprocess `go test -run TestRepoPersonaProfilePairing` invokes
// the real phasecoherence gate which reads the live agents/ and .evolve/profiles/
// directories. A no-op (empty persona file) cannot satisfy the bijection gate.
//
// RED: .evolve/profiles/preliminary-study.json exists with no paired persona →
//
//	TestRepoPersonaProfilePairing fails with "profile has no persona at agents/evolve-preliminary-study.md".
//
// GREEN: Builder creates agents/evolve-preliminary-study.md → bijection gate passes.
func TestC17_003_PhaseCoherencePasses(t *testing.T) {
	combined, code := goTestRun(t,
		"-count=1",
		"-run", "TestRepoPersonaProfilePairing",
		"./internal/phasecoherence/...",
	)
	if code != 0 {
		t.Errorf("RED: phasecoherence.TestRepoPersonaProfilePairing FAIL (exit %d).\n"+
			"Builder must create agents/evolve-preliminary-study.md to pair with\n"+
			".evolve/profiles/preliminary-study.json.\nOutput:\n%s", code, combined)
	}
}

// TestC17_004_PhaseSpecPasses verifies that
// phasespec.TestPhaseCatalog_OptionalPhasesHaveSelectMetadata passes (exit 0).
//
// BEHAVIORAL: subprocess `go test -run TestPhaseCatalog_OptionalPhasesHaveSelectMetadata`
// invokes the real phasespec gate which reads phase.json files from all phase roots.
//
// RED: .evolve/phases/preliminary-study/phase.json has no when_to_use or description →
//
//	test reports "[preliminary-study]" as a phase lacking SELECT metadata.
//
// GREEN: Builder adds when_to_use and description fields to phase.json.
func TestC17_004_PhaseSpecPasses(t *testing.T) {
	combined, code := goTestRun(t,
		"-count=1",
		"-run", "TestPhaseCatalog_OptionalPhasesHaveSelectMetadata",
		"./internal/phasespec/...",
	)
	if code != 0 {
		t.Errorf("RED: phasespec.TestPhaseCatalog_OptionalPhasesHaveSelectMetadata FAIL (exit %d).\n"+
			"Builder must add when_to_use and description fields to\n"+
			".evolve/phases/preliminary-study/phase.json.\nOutput:\n%s", code, combined)
	}
}

// ── cmd-campaign-dispatch-coverage ──────────────────────────────────────────

// TestC17_005_DispatchNoArgsTestExistsAndPasses verifies that
// TestRunCampaign_NoArgs exists in cmd_campaign_test.go and passes.
//
// BEHAVIORAL: subprocess `go test -v -run TestRunCampaign_NoArgs` — checks
// for "--- PASS: TestRunCampaign_NoArgs" in the -v output. If the test
// function does not exist, `go test -run` exits 0 but the PASS line is absent.
//
// RED: TestRunCampaign_NoArgs does not exist in cmd_campaign_test.go →
//
//	go test -v output does not contain "--- PASS: TestRunCampaign_NoArgs".
//
// GREEN: Builder adds TestRunCampaign_NoArgs which calls runCampaign([]string{}, ...)
//
//	and asserts exit=2 + stderr contains usage.
func TestC17_005_DispatchNoArgsTestExistsAndPasses(t *testing.T) {
	combined, code := goTestRun(t,
		"-v", "-count=1",
		"-run", "TestRunCampaign_NoArgs",
		"./cmd/evolve/...",
	)
	if code != 0 {
		t.Errorf("RED: go test -run TestRunCampaign_NoArgs failed (exit %d).\n"+
			"Builder must add TestRunCampaign_NoArgs to go/cmd/evolve/cmd_campaign_test.go.\nOutput:\n%s",
			code, combined)
		return
	}
	if !strings.Contains(combined, "--- PASS: TestRunCampaign_NoArgs") {
		t.Errorf("RED: TestRunCampaign_NoArgs was not found or not run.\n"+
			"(go test -v output lacks '--- PASS: TestRunCampaign_NoArgs' — function not yet defined.)\n"+
			"Builder must add:\n"+
			"  func TestRunCampaign_NoArgs(t *testing.T) {\n"+
			"      var stdout, stderr bytes.Buffer\n"+
			"      if code := runCampaign([]string{}, nil, &stdout, &stderr); code != 2 {\n"+
			"          t.Fatalf(...)\n"+
			"      }\n"+
			"  }\n"+
			"File: go/cmd/evolve/cmd_campaign_test.go\nOutput:\n%s", combined)
	}
}

// TestC17_005neg_DispatchUnknownSubTestExistsAndPasses verifies that
// TestRunCampaign_UnknownSub exists and passes (semantic diversity —
// a distinct dispatch guard case from no-args).
//
// BEHAVIORAL: same subprocess -v approach as C17_005, for a different dispatch path.
// RED: TestRunCampaign_UnknownSub does not exist yet.
// GREEN: Builder adds TestRunCampaign_UnknownSub asserting runCampaign([]string{"unknown"}, ...) exits 2.
func TestC17_005neg_DispatchUnknownSubTestExistsAndPasses(t *testing.T) {
	combined, code := goTestRun(t,
		"-v", "-count=1",
		"-run", "TestRunCampaign_UnknownSub",
		"./cmd/evolve/...",
	)
	if code != 0 {
		t.Errorf("RED: go test -run TestRunCampaign_UnknownSub failed (exit %d).\n"+
			"Builder must add TestRunCampaign_UnknownSub to cmd_campaign_test.go.\nOutput:\n%s",
			code, combined)
		return
	}
	if !strings.Contains(combined, "--- PASS: TestRunCampaign_UnknownSub") {
		t.Errorf("RED: TestRunCampaign_UnknownSub was not found or not run.\n"+
			"(go test -v output lacks '--- PASS: TestRunCampaign_UnknownSub')\n"+
			"Builder must add:\n"+
			"  func TestRunCampaign_UnknownSub(t *testing.T) {\n"+
			"      var stdout, stderr bytes.Buffer\n"+
			"      if code := runCampaign([]string{\"unknown\"}, nil, &stdout, &stderr); code != 2 {\n"+
			"          t.Fatalf(...)\n"+
			"      }\n"+
			"  }\n"+
			"File: go/cmd/evolve/cmd_campaign_test.go\nOutput:\n%s", combined)
	}
}

// TestC17_006_RenderPlanValidTestExistsAndPasses verifies that
// TestRenderCampaignPlan_Valid exists in cmd_campaign_test.go and passes.
//
// BEHAVIORAL: subprocess `go test -v -run TestRenderCampaignPlan_Valid` checks
// for "--- PASS: TestRenderCampaignPlan_Valid" in the -v output. Existing
// renderCampaignPlan tests only cover error paths (missing file, bad version);
// the success path (valid plan → exit 0 + non-empty stdout) is at 40%.
//
// RED: TestRenderCampaignPlan_Valid does not exist → PASS line absent.
// GREEN: Builder writes a temp valid campaign-plan.json and calls renderCampaignPlan,
//
//	asserts exit=0 and non-empty stdout.
func TestC17_006_RenderPlanValidTestExistsAndPasses(t *testing.T) {
	combined, code := goTestRun(t,
		"-v", "-count=1",
		"-run", "TestRenderCampaignPlan_Valid",
		"./cmd/evolve/...",
	)
	if code != 0 {
		t.Errorf("RED: go test -run TestRenderCampaignPlan_Valid failed (exit %d).\n"+
			"Builder must add TestRenderCampaignPlan_Valid to go/cmd/evolve/cmd_campaign_test.go.\nOutput:\n%s",
			code, combined)
		return
	}
	if !strings.Contains(combined, "--- PASS: TestRenderCampaignPlan_Valid") {
		t.Errorf("RED: TestRenderCampaignPlan_Valid was not found or not run.\n"+
			"(go test -v output lacks '--- PASS: TestRenderCampaignPlan_Valid')\n"+
			"Builder must add:\n"+
			"  func TestRenderCampaignPlan_Valid(t *testing.T) {\n"+
			"      path := writeCampaignTestPlan(t)\n"+
			"      var stdout, stderr bytes.Buffer\n"+
			"      if code := renderCampaignPlan(path, &stdout, &stderr); code != 0 {\n"+
			"          t.Fatalf(\"want exit 0, got %%d; stderr=%%s\", code, stderr.String())\n"+
			"      }\n"+
			"      if stdout.Len() == 0 {\n"+
			"          t.Fatal(\"renderCampaignPlan produced empty stdout on valid plan\")\n"+
			"      }\n"+
			"  }\n"+
			"File: go/cmd/evolve/cmd_campaign_test.go\nOutput:\n%s", combined)
	}
}
